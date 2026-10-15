package machine

const (
	frameworkVendor  = "UNSET-framework-sys_vendor"
	frameworkProduct = "UNSET-framework-product_name"
)

func (f Facts) Framework() bool {
	return f.Vendor == frameworkVendor && f.Product == frameworkProduct
}

func (f Facts) Profile() Profile {
	p := Profile{
		Preset:    Custom,
		CPU:       f.CPU,
		GPUs:      f.GPUs,
		Form:      Desktop,
		Backlight: f.Backlight,
		WiFi:      f.Battery,
		WLAN:      f.WLAN,
		YubiKeys:  2,
		Lane:      f.Lane,
	}
	if f.Battery {
		p.Form = Laptop
	}
	if f.Framework() {
		p.Preset, p.CPU, p.GPUs, p.Form, p.WiFi = Framework, AMD, []string{AMD}, Desktop, false
	}
	return p
}

func (p Profile) DefaultHostname() string {
	if p.Preset == Framework {
		return "costello"
	}
	return ""
}

type Firmware struct {
	Clear  string
	Enable string
}

func (p Profile) Firmware() Firmware {
	if p.Preset == Framework {
		return Firmware{
			Clear:  "press F2, choose Erase all Secure Boot Settings, and save with F10",
			Enable: "press F2, enable Secure Boot, and save with F10",
		}
	}
	return Firmware{
		Clear:  "enter firmware setup, clear the Secure Boot keys to enter Setup Mode, keep the TPM enabled, and save",
		Enable: "enter firmware setup, enable Secure Boot, and save",
	}
}
