return {
	"chrisgrieser/nvim-recorder",
	dependencies = "rcarriga/nvim-notify",
	opts = {
		slots = { "a", "s", "e", "t" },
		mapping = {
			startStopRecording = "@",
			playMacro = "@",
			switchSlot = "<C-q>",
			editMacro = "cq",
			deleteAllMacros = "dq",
		},
	},
}
