package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	librePodsService          = "me.kavishdevar.librepods"
	librePodsPath             = dbus.ObjectPath("/me/kavishdevar/librepods")
	librePodsInterface        = "me.kavishdevar.librepods.Metadata1"
	librePodsInterfaceVersion = uint32(1)
	librePodsCallTimeout      = 2 * time.Second
)

var errLibrePodsVersion = errors.New("incompatible LibrePods metadata interface")

type librePodsState struct {
	Owner                               string
	Generation                          string
	Address                             string
	Readiness                           string
	BatteryPercent                      int
	BatteryValid                        bool
	BatteryCharging                     bool
	NoiseControl                        string
	NoiseControlKnown                   bool
	NoiseControlModes                   []string
	NoiseControlModesKnown              bool
	WearState                           string
	ConversationAwarenessSupportedKnown bool
	ConversationAwarenessSupported      bool
	ConversationAwarenessEnabledKnown   bool
	ConversationAwarenessEnabled        bool
	PersonalizedVolumeSupportedKnown    bool
	PersonalizedVolumeSupported         bool
	PersonalizedVolumeEnabledKnown      bool
	PersonalizedVolumeEnabled           bool
	HiResMicSupportedKnown              bool
	HiResMicSupported                   bool
	HiResMicEnabledKnown                bool
	HiResMicEnabled                     bool
}

type librePodsUpdate struct {
	state *librePodsState
}

type librePodsCommand struct {
	owner      string
	generation string
	target     string
	token      uint64
}

type librePodsResult struct {
	command librePodsCommand
	err     error
}

type librePodsSnapshot struct {
	epoch    uint64
	revision uint64
	owner    string
	props    map[string]dbus.Variant
	err      error
}

func watchLibrePods(ctx context.Context, updates chan<- librePodsUpdate, commands <-chan librePodsCommand, results chan<- librePodsResult) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		failLibrePods(ctx, updates, fmt.Errorf("connect session bus: %w", err))
		return
	}
	defer conn.Close()

	ownerMatch := []dbus.MatchOption{
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, librePodsService),
	}
	if err := conn.AddMatchSignal(ownerMatch...); err != nil {
		failLibrePods(ctx, updates, fmt.Errorf("watch LibrePods owner: %w", err))
		return
	}
	defer conn.RemoveMatchSignal(ownerMatch...)

	signals := make(chan *dbus.Signal, 32)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	owner := busOwner(ctx, conn, librePodsService)
	var signalOwner string
	var epoch uint64
	var revision uint64
	var snapshotPending bool
	var snapshotWanted bool
	snapshots := make(chan librePodsSnapshot, 4)

	removeOwnerMatch := func() {
		if signalOwner == "" {
			return
		}
		_ = conn.RemoveMatchSignal(librePodsSignalMatch(signalOwner)...)
		signalOwner = ""
	}
	defer removeOwnerMatch()

	startSnapshot := func() {
		if owner == "" || snapshotPending {
			snapshotWanted = snapshotPending
			return
		}
		snapshotPending = true
		snapshotWanted = false
		go readLibrePodsSnapshot(ctx, conn, owner, epoch, revision, snapshots)
	}

	activateOwner := func(next string) {
		removeOwnerMatch()
		epoch++
		owner = next
		snapshotPending = false
		snapshotWanted = false
		sendLibrePodsUpdate(ctx, updates, librePodsUpdate{})
		if owner == "" {
			return
		}
		signalOwner = owner
		if err := conn.AddMatchSignal(librePodsSignalMatch(owner)...); err != nil {
			failLibrePods(ctx, updates, fmt.Errorf("watch LibrePods metadata: %w", err))
			removeOwnerMatch()
			return
		}
		startSnapshot()
	}

	activateOwner(owner)
	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-signals:
			if !ok {
				failLibrePods(ctx, updates, errors.New("session bus signal stream closed"))
				return
			}
			if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" {
				if next, ok := changedBusOwner(signal, librePodsService); ok {
					activateOwner(next)
				}
				continue
			}
			if !currentLibrePodsSignal(signal, owner) {
				continue
			}
			revision++
			props, ok := librePodsChangedProperties(signal)
			if snapshotPending {
				snapshotWanted = true
			}
			if !ok {
				startSnapshot()
				continue
			}
			state, err := parseLibrePodsState(owner, props)
			if err != nil {
				failLibrePods(ctx, updates, err)
				startSnapshot()
				continue
			}
			sendLibrePodsUpdate(ctx, updates, librePodsUpdate{state: state})
		case snapshot := <-snapshots:
			if snapshot.epoch != epoch || snapshot.owner != owner {
				continue
			}
			snapshotPending = false
			if !currentLibrePodsSnapshot(snapshot, epoch, revision, owner) {
				// A signal supplied a newer complete state while GetAll was in flight.
				// Publishing this result would briefly resurrect the old generation.
			} else if snapshot.err != nil {
				failLibrePods(ctx, updates, snapshot.err)
			} else {
				state, err := parseLibrePodsState(owner, snapshot.props)
				if err != nil {
					failLibrePods(ctx, updates, err)
				} else {
					sendLibrePodsUpdate(ctx, updates, librePodsUpdate{state: state})
				}
			}
			if snapshotWanted {
				startSnapshot()
			}
		case command := <-commands:
			if command.owner != owner || owner == "" {
				sendLibrePodsResult(ctx, results, librePodsResult{command: command, err: errors.New("LibrePods service owner changed")})
				continue
			}
			go selectLibrePodsNoiseControl(ctx, conn, command, results)
		}
	}
}

func busOwner(ctx context.Context, conn *dbus.Conn, name string) string {
	callCtx, cancel := context.WithTimeout(ctx, librePodsCallTimeout)
	defer cancel()
	var owner string
	if err := conn.BusObject().CallWithContext(callCtx, "org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner); err != nil {
		return ""
	}
	return owner
}

func librePodsSignalMatch(owner string) []dbus.MatchOption {
	return []dbus.MatchOption{
		dbus.WithMatchSender(owner),
		dbus.WithMatchObjectPath(librePodsPath),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, librePodsInterface),
	}
}

func changedBusOwner(signal *dbus.Signal, name string) (string, bool) {
	if signal.Sender != "org.freedesktop.DBus" || len(signal.Body) != 3 {
		return "", false
	}
	changed, nameOK := signal.Body[0].(string)
	next, nextOK := signal.Body[2].(string)
	return next, nameOK && nextOK && changed == name
}

func currentLibrePodsSignal(signal *dbus.Signal, owner string) bool {
	return owner != "" && signal.Sender == owner && signal.Path == librePodsPath && signal.Name == "org.freedesktop.DBus.Properties.PropertiesChanged"
}

func currentLibrePodsSnapshot(snapshot librePodsSnapshot, epoch, revision uint64, owner string) bool {
	return snapshot.epoch == epoch && snapshot.revision == revision && snapshot.owner == owner
}

func librePodsChangedProperties(signal *dbus.Signal) (map[string]dbus.Variant, bool) {
	if len(signal.Body) != 3 {
		return nil, false
	}
	iface, ifaceOK := signal.Body[0].(string)
	props, propsOK := signal.Body[1].(map[string]dbus.Variant)
	invalidated, invalidatedOK := signal.Body[2].([]string)
	return props, ifaceOK && propsOK && invalidatedOK && iface == librePodsInterface && len(invalidated) == 0
}

func readLibrePodsSnapshot(ctx context.Context, conn *dbus.Conn, owner string, epoch, revision uint64, snapshots chan<- librePodsSnapshot) {
	result := librePodsSnapshot{epoch: epoch, revision: revision, owner: owner}
	callCtx, cancel := context.WithTimeout(ctx, librePodsCallTimeout)
	defer cancel()
	result.err = conn.Object(owner, librePodsPath).CallWithContext(callCtx, "org.freedesktop.DBus.Properties.GetAll", 0, librePodsInterface).Store(&result.props)
	select {
	case snapshots <- result:
	case <-ctx.Done():
	}
}

func selectLibrePodsNoiseControl(ctx context.Context, conn *dbus.Conn, command librePodsCommand, results chan<- librePodsResult) {
	callCtx, cancel := context.WithTimeout(ctx, librePodsCallTimeout)
	defer cancel()
	err := conn.Object(command.owner, librePodsPath).CallWithContext(
		callCtx,
		librePodsInterface+".SelectNoiseControl",
		0,
		command.generation,
		command.target,
	).Err
	sendLibrePodsResult(ctx, results, librePodsResult{command: command, err: err})
}

func sendLibrePodsUpdate(ctx context.Context, updates chan<- librePodsUpdate, update librePodsUpdate) {
	select {
	case updates <- update:
	case <-ctx.Done():
	}
}

func failLibrePods(ctx context.Context, updates chan<- librePodsUpdate, err error) {
	reportLibrePodsError(err)
	sendLibrePodsUpdate(ctx, updates, librePodsUpdate{})
}

func sendLibrePodsResult(ctx context.Context, results chan<- librePodsResult, result librePodsResult) {
	select {
	case results <- result:
	case <-ctx.Done():
	}
}

func reportLibrePodsError(err error) {
	if errors.Is(err, errLibrePodsVersion) {
		fmt.Fprintf(os.Stderr, "ewwd: LibrePods metadata rejected: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "ewwd: LibrePods metadata unavailable: %v\n", err)
}

func parseLibrePodsState(owner string, props map[string]dbus.Variant) (*librePodsState, error) {
	version, ok := property[uint32](props, "InterfaceVersion")
	if !ok {
		return nil, errors.New("LibrePods InterfaceVersion is missing or malformed")
	}
	if version != librePodsInterfaceVersion {
		return nil, fmt.Errorf("%w: got %d, require %d", errLibrePodsVersion, version, librePodsInterfaceVersion)
	}

	state := &librePodsState{Owner: owner}
	var valid bool
	if state.Generation, valid = property[string](props, "SessionGeneration"); !valid || state.Generation == "" {
		return nil, errors.New("LibrePods SessionGeneration is missing or empty")
	}
	if state.Address, valid = property[string](props, "DeviceAddress"); !valid {
		return nil, errors.New("LibrePods DeviceAddress is missing or malformed")
	}
	if state.Readiness, valid = property[string](props, "Readiness"); !valid || !slices.Contains([]string{"initializing", "ready", "disconnected", "failed"}, state.Readiness) {
		return nil, fmt.Errorf("invalid LibrePods Readiness %q", state.Readiness)
	}
	if state.Readiness == "ready" && state.Address == "" {
		return nil, errors.New("ready LibrePods snapshot has no DeviceAddress")
	}
	battery, valid := property[byte](props, "BatteryPercent")
	if !valid || battery > 100 {
		return nil, errors.New("LibrePods BatteryPercent is missing or invalid")
	}
	state.BatteryPercent = int(battery)

	boolProperties := []struct {
		name string
		to   *bool
	}{
		{"BatteryValid", &state.BatteryValid},
		{"BatteryCharging", &state.BatteryCharging},
		{"NoiseControlKnown", &state.NoiseControlKnown},
		{"NoiseControlModesKnown", &state.NoiseControlModesKnown},
		{"ConversationAwarenessSupportedKnown", &state.ConversationAwarenessSupportedKnown},
		{"ConversationAwarenessSupported", &state.ConversationAwarenessSupported},
		{"ConversationAwarenessEnabledKnown", &state.ConversationAwarenessEnabledKnown},
		{"ConversationAwarenessEnabled", &state.ConversationAwarenessEnabled},
		{"PersonalizedVolumeSupportedKnown", &state.PersonalizedVolumeSupportedKnown},
		{"PersonalizedVolumeSupported", &state.PersonalizedVolumeSupported},
		{"PersonalizedVolumeEnabledKnown", &state.PersonalizedVolumeEnabledKnown},
		{"PersonalizedVolumeEnabled", &state.PersonalizedVolumeEnabled},
		{"HiResMicSupportedKnown", &state.HiResMicSupportedKnown},
		{"HiResMicSupported", &state.HiResMicSupported},
		{"HiResMicEnabledKnown", &state.HiResMicEnabledKnown},
		{"HiResMicEnabled", &state.HiResMicEnabled},
	}
	for _, field := range boolProperties {
		value, ok := property[bool](props, field.name)
		if !ok {
			return nil, fmt.Errorf("LibrePods %s is missing or malformed", field.name)
		}
		*field.to = value
	}

	if state.NoiseControl, valid = property[string](props, "NoiseControl"); !valid {
		return nil, errors.New("LibrePods NoiseControl is missing or malformed")
	}
	if state.NoiseControlModes, valid = property[[]string](props, "NoiseControlModes"); !valid {
		return nil, errors.New("LibrePods NoiseControlModes is missing or malformed")
	}
	if state.NoiseControlModesKnown {
		for i, mode := range state.NoiseControlModes {
			if !validNoiseMode(mode) || slices.Contains(state.NoiseControlModes[:i], mode) {
				return nil, fmt.Errorf("invalid LibrePods noise-control modes %v", state.NoiseControlModes)
			}
		}
	} else if len(state.NoiseControlModes) != 0 {
		return nil, errors.New("unknown LibrePods NoiseControlModes has values")
	}
	if !state.NoiseControlKnown && state.NoiseControl != "" {
		return nil, errors.New("unknown LibrePods NoiseControl has a value")
	}
	if state.NoiseControlKnown && state.NoiseControl != "" && (!validNoiseMode(state.NoiseControl) || (state.NoiseControlModesKnown && !slices.Contains(state.NoiseControlModes, state.NoiseControl))) {
		return nil, fmt.Errorf("invalid LibrePods NoiseControl %q", state.NoiseControl)
	}
	if err := validateLibrePodsCapability("ConversationAwareness", state.ConversationAwarenessSupportedKnown, state.ConversationAwarenessSupported, state.ConversationAwarenessEnabledKnown, state.ConversationAwarenessEnabled); err != nil {
		return nil, err
	}
	if err := validateLibrePodsCapability("PersonalizedVolume", state.PersonalizedVolumeSupportedKnown, state.PersonalizedVolumeSupported, state.PersonalizedVolumeEnabledKnown, state.PersonalizedVolumeEnabled); err != nil {
		return nil, err
	}
	if err := validateLibrePodsCapability("HiResMic", state.HiResMicSupportedKnown, state.HiResMicSupported, state.HiResMicEnabledKnown, state.HiResMicEnabled); err != nil {
		return nil, err
	}
	if state.WearState, valid = property[string](props, "WearState"); !valid || !slices.Contains([]string{"unknown", "worn", "not_worn"}, state.WearState) {
		return nil, fmt.Errorf("invalid LibrePods WearState %q", state.WearState)
	}
	return state, nil
}

func validateLibrePodsCapability(name string, supportedKnown, supported, enabledKnown, enabled bool) error {
	if !supportedKnown && (supported || enabledKnown || enabled) {
		return fmt.Errorf("unknown LibrePods %s capability has values", name)
	}
	if !supported && (enabledKnown || enabled) {
		return fmt.Errorf("unsupported LibrePods %s capability has enabled state", name)
	}
	if !enabledKnown && enabled {
		return fmt.Errorf("unknown LibrePods %s enabled state has a value", name)
	}
	return nil
}

func property[T any](props map[string]dbus.Variant, name string) (T, bool) {
	var zero T
	variant, ok := props[name]
	if !ok {
		return zero, false
	}
	value, ok := variant.Value().(T)
	return value, ok
}

func validNoiseMode(mode string) bool {
	return slices.Contains([]string{"anc", "transparency", "adaptive", "off"}, mode)
}
