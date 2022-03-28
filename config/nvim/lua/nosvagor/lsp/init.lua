local status_ok, _ = pcall(require, "lspconfig")
if not status_ok then
  return
end

require "nosvagor.lsp.lsp-installer"
require "nosvagor.lsp.handlers".setup()
require "nosvagor.lsp.null-ls"
