return {
	"mfussenegger/nvim-lint",
	event = { "BufReadPre", "BufNewFile" },
	config = function()
		local lint = require("lint")
		local oxlint_filetypes = {
			javascript = true,
			javascriptreact = true,
			typescript = true,
			typescriptreact = true,
		}
		local oxlint_root_markers = { ".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.json", "oxlint.jsonc" }

		local function oxlint_root(bufnr)
			local filename = vim.api.nvim_buf_get_name(bufnr)
			return filename ~= "" and vim.fs.root(filename, oxlint_root_markers) or nil
		end

		local oxlint = require("lint.linters.oxlint")
		lint.linters.oxlint = function()
			return vim.tbl_deep_extend("force", oxlint, {
				cwd = oxlint_root(0),
			})
		end

		lint.linters_by_ft = {
			zsh = { "zsh" },
			dockerfile = { "hadolint" },
		}
		vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost" }, {
			group = vim.api.nvim_create_augroup("nvim-lint", { clear = true }),
			callback = function(args)
				if vim.bo[args.buf].buftype == "" then
					vim.api.nvim_buf_call(args.buf, function()
						if oxlint_filetypes[vim.bo.filetype] and oxlint_root(args.buf) then
							lint.try_lint("oxlint")
						end
						lint.try_lint()
					end)
				end
			end,
		})
	end,
}
