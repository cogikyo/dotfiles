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
		local golangci_root_markers = { ".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json" }

		local function root(bufnr, markers)
			local filename = vim.api.nvim_buf_get_name(bufnr)
			return filename ~= "" and vim.fs.root(filename, markers) or nil
		end

		local oxlint = require("lint.linters.oxlint")
		lint.linters.oxlint = function()
			return vim.tbl_deep_extend("force", oxlint, {
				cwd = root(0, oxlint_root_markers),
				append_fname = false,
				args = { "--format", "github", vim.api.nvim_buf_get_name(0) },
			})
		end

		lint.linters.golangcilint = function()
			-- nvim-lint caches file/package scope from cwd at module load.
			local filename = vim.api.nvim_buf_get_name(0)
			return vim.tbl_deep_extend("force", require("lint.linters.golangcilint"), {
				cwd = root(0, golangci_root_markers),
				args = {
					"run",
					"--output.json.path=stdout",
					"--output.text.path=",
					"--output.tab.path=",
					"--output.html.path=",
					"--output.checkstyle.path=",
					"--output.code-climate.path=",
					"--output.junit-xml.path=",
					"--output.teamcity.path=",
					"--output.sarif.path=",
					"--issues-exit-code=0",
					"--show-stats=false",
					"--path-mode=abs",
					vim.fs.root(filename, "go.mod") and vim.fs.dirname(filename) or filename,
				},
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
						if oxlint_filetypes[vim.bo.filetype] and root(args.buf, oxlint_root_markers) then
							lint.try_lint("oxlint")
						end
						if vim.bo.filetype == "go" and root(args.buf, golangci_root_markers) then
							lint.try_lint("golangcilint")
						end
						lint.try_lint()
					end)
				end
			end,
		})
	end,
}
