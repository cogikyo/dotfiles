return {
	"brianhuster/live-preview.nvim",
	dependencies = { "MeanderingProgrammer/render-markdown.nvim" },
	ft = { "markdown" },
	cmd = "LivePreview",
	config = function()
		require("livepreview.config").set({ dynamic_root = true })

		local kinds = {
			RenderMarkdownInfo = "note",
			RenderMarkdownSuccess = "tip",
			RenderMarkdownHint = "important",
			RenderMarkdownWarn = "warning",
			RenderMarkdownError = "caution",
		}
		local callouts = {}
		for _, callout in pairs(require("render-markdown.state").config.callout) do
			callouts[callout.raw:lower()] = {
				kind = kinds[callout.highlight],
				title = callout.rendered:match("^%S+%s+(.+)$") or callout.rendered,
			}
		end

		local file = assert(io.open(vim.fn.stdpath("config") .. "/preview/callouts.js"))
		local script = ("<script>{\nconst callouts = %s\n%s}</script>\n"):format(
			vim.json.encode(callouts),
			file:read("*a")
		)
		file:close()

		-- Private template hook: callouts.js must intercept window.markdownit before live-preview creates its renderer.
		local template = require("livepreview.template")
		local md2html = template.md2html
		template.md2html = function(md)
			local html = md2html(md)
			local head = html:find("</head>", 1, true)
			if not head then
				vim.schedule(function()
					vim.notify("live-preview: page has no </head>; callouts render as quotes", vim.log.levels.ERROR)
				end)
				return html
			end
			return html:sub(1, head - 1) .. script .. html:sub(head)
		end
	end,
}
