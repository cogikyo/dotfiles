local slots = { "a", "s", "e", "t" }

return {
	"chrisgrieser/nvim-recorder",
	dependencies = "rcarriga/nvim-notify",
	opts = {
		slots = slots,
		mapping = {
			startStopRecording = "Q",
			playMacro = "@",
			switchSlot = "<C-m>",
			editMacro = "cQ",
			deleteAllMacros = "dQ",
		},
	},
	config = function(_, opts)
		require("recorder").setup(opts)
		vim.keymap.set("n", "dm", function()
			for _, slot in ipairs(slots) do
				vim.fn.setreg(slot, "")
			end
			vim.cmd("wshada!")
			vim.notify("Cleared all macros", vim.log.levels.INFO)
		end, { desc = "Delete all macros" })
	end,
}
