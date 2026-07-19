package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	bluezService          = "org.bluez"
	bluezDeviceInterface  = "org.bluez.Device1"
	bluezBatteryInterface = "org.bluez.Battery1"
	bluezOperationTimeout = 12 * time.Second
)

type BluetoothState struct {
	Status         string `json:"status"`
	Name           string `json:"name"`
	BatteryPresent bool   `json:"battery_present"`
	BatteryPercent int    `json:"battery_percent"`
}

type bluetoothRequest struct {
	action string
	reply  chan error
}

type bluetoothScan struct {
	generation uint64
	path       dbus.ObjectPath
	state      BluetoothState
	err        error
}

type bluetoothResult struct {
	generation uint64
	token      uint64
	err        error
}

type bluetoothOperation struct {
	token     uint64
	connect   bool
	reconnect bool
}

type Bluetooth struct {
	state    StateSetter
	address  string
	requests chan bluetoothRequest
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
	notify   func(data any)
	last     BluetoothState
	hasLast  bool
}

func NewBluetooth(state StateSetter, address string) Provider {
	return &Bluetooth{
		state:    state,
		address:  address,
		requests: make(chan bluetoothRequest),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (b *Bluetooth) Name() string { return "bluetooth" }

func (b *Bluetooth) Start(ctx context.Context, notify func(data any)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(b.stopped)
	b.notify = notify
	b.publish(BluetoothState{Status: "unknown"})

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus: %w", err)
	}
	defer conn.Close()

	ownerMatch := []dbus.MatchOption{
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, bluezService),
	}
	if err := conn.AddMatchSignal(ownerMatch...); err != nil {
		return fmt.Errorf("watch BlueZ owner: %w", err)
	}
	defer conn.RemoveMatchSignal(ownerMatch...)

	signals := make(chan *dbus.Signal, 64)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	owner := bluezOwner(ctx, conn)
	var generation uint64
	var path dbus.ObjectPath
	var signalOwner string
	var scanPending bool
	var scanWanted bool
	var pending *bluetoothOperation
	var token uint64
	var deadline *time.Timer
	var deadlineC <-chan time.Time
	scans := make(chan bluetoothScan, 4)
	results := make(chan bluetoothResult, 4)

	removeOwnerMatches := func() {
		if signalOwner == "" {
			return
		}
		for _, match := range bluezSignalMatches(signalOwner) {
			_ = conn.RemoveMatchSignal(match...)
		}
		signalOwner = ""
	}
	defer removeOwnerMatches()

	startScan := func() {
		if owner == "" || scanPending {
			scanWanted = scanPending
			return
		}
		scanPending = true
		scanWanted = false
		go scanBluetooth(ctx, conn, owner, b.address, generation, scans)
	}

	activateOwner := func(next string) {
		removeOwnerMatches()
		generation++
		owner = next
		path = ""
		pending = nil
		if deadline != nil {
			deadline.Stop()
		}
		deadlineC = nil
		b.publish(BluetoothState{Status: "unknown"})
		if owner == "" {
			return
		}
		signalOwner = owner
		for _, match := range bluezSignalMatches(owner) {
			if err := conn.AddMatchSignal(match...); err != nil {
				fmt.Fprintf(os.Stderr, "ewwd: BlueZ signal match: %v\n", err)
				removeOwnerMatches()
				return
			}
		}
		startScan()
	}

	activateOwner(owner)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-b.stop:
			return nil
		case signal, ok := <-signals:
			if !ok {
				b.publish(BluetoothState{Status: "unknown"})
				return errors.New("system bus signal stream closed")
			}
			if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" {
				if next, ok := changedBluezOwner(signal); ok {
					activateOwner(next)
				}
				continue
			}
			if signal.Sender == owner && relevantBluezSignal(signal, path) {
				startScan()
			}
		case scan := <-scans:
			scanPending = false
			if scan.generation == generation && scan.err == nil {
				path = scan.path
				snapshot := scan.state
				if pending != nil {
					if pending.reconnect && snapshot.Status == "disconnected" {
						token++
						pending.token = token
						pending.reconnect = false
						go runBluetoothOperation(ctx, conn, owner, path, generation, *pending, results)
						snapshot.Status = "connecting"
						snapshot.BatteryPresent = false
						snapshot.BatteryPercent = 0
					} else if resolvedBluetoothOperation(*pending, snapshot) {
						pending = nil
						if deadline != nil {
							deadline.Stop()
						}
						deadlineC = nil
					} else if pending.connect {
						snapshot.Status = "connecting"
						snapshot.BatteryPresent = false
						snapshot.BatteryPercent = 0
					}
				}
				b.publish(snapshot)
			} else if scan.generation == generation {
				b.publish(BluetoothState{Status: "unknown"})
			}
			if scanWanted {
				startScan()
			}
		case result := <-results:
			if pending == nil || result.generation != generation || result.token != pending.token {
				continue
			}
			if result.err != nil {
				pending = nil
				if deadline != nil {
					deadline.Stop()
				}
				deadlineC = nil
				fmt.Fprintf(os.Stderr, "ewwd: Bluetooth operation: %v\n", result.err)
				startScan()
			}
		case <-deadlineC:
			pending = nil
			deadlineC = nil
			startScan()
		case req := <-b.requests:
			if pending != nil {
				req.reply <- errors.New("Bluetooth connection operation already active")
				continue
			}
			if owner == "" || path == "" || b.last.Status == "unknown" {
				req.reply <- errors.New("tracked Bluetooth device is unavailable")
				continue
			}
			if req.action != "toggle" && req.action != "reconnect" {
				req.reply <- fmt.Errorf("unknown Bluetooth action: %s", req.action)
				continue
			}

			connect := b.last.Status != "connected" || req.action == "reconnect"
			token++
			pending = &bluetoothOperation{
				token:     token,
				connect:   connect,
				reconnect: req.action == "reconnect" && b.last.Status == "connected",
			}
			if connect {
				snapshot := b.last
				snapshot.Status = "connecting"
				snapshot.BatteryPresent = false
				snapshot.BatteryPercent = 0
				b.publish(snapshot)
			}
			if deadline == nil {
				deadline = time.NewTimer(bluezOperationTimeout)
			} else {
				deadline.Reset(bluezOperationTimeout)
			}
			deadlineC = deadline.C
			go runBluetoothOperation(ctx, conn, owner, path, generation, *pending, results)
			req.reply <- nil
		}
	}
}

func (b *Bluetooth) Stop() error {
	b.stopOnce.Do(func() { close(b.stop) })
	return nil
}

func (b *Bluetooth) publish(snapshot BluetoothState) {
	if b.hasLast && snapshot == b.last {
		return
	}
	b.last = snapshot
	b.hasLast = true
	b.state.Set("bluetooth", &snapshot)
	if b.notify != nil {
		b.notify(&snapshot)
	}
}

func (b *Bluetooth) HandleAction(args []string) (string, error) {
	if len(args) != 1 {
		return "", errors.New("Bluetooth action required: toggle or reconnect")
	}
	reply := make(chan error, 1)
	select {
	case b.requests <- bluetoothRequest{action: args[0], reply: reply}:
	case <-b.stopped:
		return "", errors.New("Bluetooth provider is stopped")
	}
	select {
	case err := <-reply:
		if err != nil {
			return "", err
		}
		return "ok", nil
	case <-b.stopped:
		return "", errors.New("Bluetooth provider is stopped")
	}
}

func bluezOwner(ctx context.Context, conn *dbus.Conn) string {
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var owner string
	if err := conn.BusObject().CallWithContext(callCtx, "org.freedesktop.DBus.GetNameOwner", 0, bluezService).Store(&owner); err != nil {
		return ""
	}
	return owner
}

func bluezSignalMatches(owner string) [][]dbus.MatchOption {
	return [][]dbus.MatchOption{
		{
			dbus.WithMatchSender(owner),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			dbus.WithMatchMember("PropertiesChanged"),
		},
		{
			dbus.WithMatchSender(owner),
			dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager"),
			dbus.WithMatchMember("InterfacesAdded"),
		},
		{
			dbus.WithMatchSender(owner),
			dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager"),
			dbus.WithMatchMember("InterfacesRemoved"),
		},
	}
}

func changedBluezOwner(signal *dbus.Signal) (string, bool) {
	if len(signal.Body) != 3 {
		return "", false
	}
	name, nameOK := signal.Body[0].(string)
	next, nextOK := signal.Body[2].(string)
	return next, nameOK && nextOK && name == bluezService
}

func relevantBluezSignal(signal *dbus.Signal, trackedPath dbus.ObjectPath) bool {
	switch signal.Name {
	case "org.freedesktop.DBus.Properties.PropertiesChanged":
		if len(signal.Body) == 0 {
			return false
		}
		iface, ok := signal.Body[0].(string)
		if !ok {
			return false
		}
		if iface == bluezDeviceInterface {
			return true
		}
		return iface == bluezBatteryInterface && belongsToDevice(signal.Path, trackedPath)
	case "org.freedesktop.DBus.ObjectManager.InterfacesAdded":
		if len(signal.Body) < 2 {
			return false
		}
		interfaces, ok := signal.Body[1].(map[string]map[string]dbus.Variant)
		return ok && (interfaces[bluezDeviceInterface] != nil || interfaces[bluezBatteryInterface] != nil)
	case "org.freedesktop.DBus.ObjectManager.InterfacesRemoved":
		if len(signal.Body) < 2 {
			return false
		}
		interfaces, ok := signal.Body[1].([]string)
		if !ok {
			return false
		}
		for _, iface := range interfaces {
			if iface == bluezDeviceInterface || iface == bluezBatteryInterface {
				return true
			}
		}
	}
	return false
}

func scanBluetooth(ctx context.Context, conn *dbus.Conn, owner, address string, generation uint64, results chan<- bluetoothScan) {
	result := bluetoothScan{generation: generation}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	managed := make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)
	result.err = conn.Object(owner, dbus.ObjectPath("/")).CallWithContext(callCtx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&managed)
	if result.err == nil {
		result.path, result.state = bluetoothSnapshot(managed, address)
	}
	select {
	case results <- result:
	case <-ctx.Done():
	}
}

func bluetoothSnapshot(objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant, address string) (dbus.ObjectPath, BluetoothState) {
	state := BluetoothState{Status: "disconnected"}
	var devicePath dbus.ObjectPath
	for path, interfaces := range objects {
		device := interfaces[bluezDeviceInterface]
		if device == nil || !strings.EqualFold(variantString(device["Address"]), address) {
			continue
		}
		devicePath = path
		state.Name = variantString(device["Name"])
		if state.Name == "" {
			state.Name = variantString(device["Alias"])
		}
		if connected, ok := device["Connected"].Value().(bool); ok && connected {
			state.Status = "connected"
		}
		break
	}
	if state.Status != "connected" {
		return devicePath, state
	}
	for path, interfaces := range objects {
		battery := interfaces[bluezBatteryInterface]
		if battery == nil || !belongsToDevice(path, devicePath) {
			continue
		}
		if percent, ok := battery["Percentage"].Value().(byte); ok {
			state.BatteryPresent = true
			state.BatteryPercent = int(percent)
			break
		}
	}
	return devicePath, state
}

func variantString(value dbus.Variant) string {
	text, _ := value.Value().(string)
	return text
}

func belongsToDevice(path, devicePath dbus.ObjectPath) bool {
	if devicePath == "" {
		return false
	}
	return path == devicePath || strings.HasPrefix(string(path), string(devicePath)+"/")
}

func resolvedBluetoothOperation(operation bluetoothOperation, snapshot BluetoothState) bool {
	if operation.reconnect {
		return false
	}
	return (operation.connect && snapshot.Status == "connected") || (!operation.connect && snapshot.Status == "disconnected")
}

func runBluetoothOperation(ctx context.Context, conn *dbus.Conn, owner string, path dbus.ObjectPath, generation uint64, operation bluetoothOperation, results chan<- bluetoothResult) {
	result := bluetoothResult{generation: generation, token: operation.token}
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	device := conn.Object(owner, path)

	if operation.reconnect {
		result.err = device.CallWithContext(operationCtx, bluezDeviceInterface+".Disconnect", 0).Err
	} else if operation.connect {
		result.err = device.CallWithContext(operationCtx, bluezDeviceInterface+".Connect", 0).Err
	} else {
		result.err = device.CallWithContext(operationCtx, bluezDeviceInterface+".Disconnect", 0).Err
	}

	select {
	case results <- result:
	case <-ctx.Done():
	}
}
