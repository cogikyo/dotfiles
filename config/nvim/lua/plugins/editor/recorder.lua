return {
	"chrisgrieser/nvim-recorder",
	dependencies = "rcarriga/nvim-notify",
	opts = {
		slots = { "a", "s", "e", "t" },
		mapping = {
			startStopRecording = "M",
			playMacro = "@",
			switchSlot = "<C-m>",
			editMacro = "cm",
			deleteAllMacros = "dm",
		},
	},
}
