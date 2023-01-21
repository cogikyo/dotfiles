-- ============================================================================
-- 🛠️ Simple Setups: ⮯ {{{
require('mason').setup()
require('neodev').setup()
require('fidget').setup()
require('Comment').setup()
require("lsp-file-operations").setup()
require("nvim-web-devicons").setup({ default = true, })
require('trim').setup({
  disable = {},
  patterns = {
    -- replace multiple blank lines with a single line ⮯
    [[%s/\(\n\n\n\)\n\+/\1/]],
  },
})
-- }}} ⮭
-- ============================================================================

-- 🎨 Colorscheme: ⮯ {{{
vim.cmd [[
  colorscheme catppuccin-macchiato
  highlight ColorColumn guibg=#222536
  highlight MsgArea guifg=#565f89
  highlight Folded guibg=#24283b guifg=#565f89
]]
-- }}} ⮭

-- 🗂️ Nvim-Tree ⮯ {{{
require("nvim-tree").setup({
  disable_netrw = true,
  hijack_netrw = true,
  open_on_setup = false,
  ignore_buffer_on_setup = false,
  ignore_ft_on_setup = {},
  ignore_buf_on_tab_change = {},
  auto_reload_on_write = true,
  create_in_closed_folder = false,
  open_on_tab = false,
  sort_by = "name",
  hijack_unnamed_buffer_when_opening = false,
  hijack_cursor = true,
  root_dirs = {},
  prefer_startup_root = false,
  sync_root_with_cwd = false,
  respect_buf_cwd = false,
  hijack_directories = {
    enable = true,
    auto_open = true,
  },
  update_focused_file = {
    enable = true,
    update_cwd = false,
    ignore_list = {},
  },
  system_open = {
    cmd = nil,
    args = {},
  },
  diagnostics = {
    enable = true,
    debounce_delay = 50,
    show_on_dirs = false,
    icons = {
      hint = "",
      info = "",
      warning = "",
      error = "",
    },
  },
  git = {
    enable = true,
    ignore = true,
    show_on_dirs = true,
    timeout = 420,
  },
  filesystem_watchers = {
    enable = true,
    debounce_delay = 50,
  },
  on_attach = "disable",
  remove_keymaps = true,
  select_prompts = false,
  view = {
    adaptive_size = true,
    centralize_selection = false,
    hide_root_folder = false,
    side = "left",
    preserve_window_proportions = false,
    number = true,
    relativenumber = true,
    signcolumn = "yes",
    mappings = {
      custom_only = true,
      list = {
        { key = { "<CR>", "o" }, action = "edit" },
        { key = "]]", action = "cd" },
        { key = "<C-v>", action = "vsplit" },
        { key = "<C-h>", action = "split" },
        { key = "<C-t>", action = "tabnew" },
        { key = "<Up>", action = "prev_sibling" },
        { key = "<Down>", action = "next_sibling" },
        { key = "P", action = "parent_node" },
        { key = "h", action = "close_node" },
        { key = "K", action = "first_sibling" },
        { key = "J", action = "last_sibling" },
        { key = "I", action = "toggle_git_ignored" },
        { key = ".", action = "toggle_dotfiles" },
        { key = "<C-r>", action = "refresh" },
        { key = "t", action = "create" },
        { key = "d", action = "trash" },
        { key = "D", action = "remove" },
        { key = "r", action = "rename" },
        { key = "R", action = "full_rename" },
        { key = "x", action = "cut" },
        { key = "y", action = "copy" },
        { key = "p", action = "paste" },
        { key = "Y", action = "copy_path" },
        { key = "gy", action = "copy_absolute_path" },
        { key = "[c", action = "prev_git_item" },
        { key = "]c", action = "next_git_item" },
        { key = "-", action = "dir_up" },
        { key = "O", action = "system_open" },
        { key = "q", action = "close" },
        { key = "?", action = "toggle_help" },
        { key = "W", action = "collapse_all" },
        { key = "S", action = "search_node" },
        { key = "!", action = "run_file_command" },
        { key = "<C-k>", action = "toggle_file_info" },
        { key = "U", action = "toggle_custom" },
      },
    },
    float = {
      enable = true,
      open_win_config = {
        relative = "editor",
        border = "rounded",
        row = 3,
        col = 69,
      },
    },
  },
  renderer = {
    add_trailing = false,
    group_empty = true,
    full_name = false,
    highlight_git = true,
    highlight_opened_files = "none",
    root_folder_modifier = ":~",
    indent_width = 2,
    indent_markers = {
      enable = true,
      inline_arrows = true,
      icons = {
        corner = "└╾ ",
        edge = "│ ",
        item = "├",
        none = "",
      },
    },
    icons = {
      webdev_colors = true,
      git_placement = "before",
      padding = " ",
      symlink_arrow = " ➛ ",
      show = {
        file = true,
        folder = true,
        folder_arrow = true,
        git = true,
      },
      glyphs = {
        default = "",
        symlink = "",
        git = {
          unstaged = "🞊",
          staged = "",
          unmerged = "",
          renamed = "",
          deleted = "",
          untracked = "⧂",
          ignored = "",
        },
        folder = {
          arrow_open = "",
          arrow_closed = "",
          default = "",
          open = "",
          empty = "",
          empty_open = "",
          symlink = "",
          symlink_open = "",
        },
      }
    },
    special_files = {
      ["Cargo.toml"] = true,
      Makefile = true,
      ["README.md"] = true,
      ["readme.md"] = true,
    },
    symlink_destination = true,
  },
  filters = {
    dotfiles = false,
    custom = { "^\\.git" },
    exclude = {},
  },
  trash = {
    cmd = "trash-put",
    require_confirm = true,
  },
  actions = {
    change_dir = {
      enable = true,
      global = false,
      restrict_above_cwd = false,
    },
    open_file = {
      quit_on_open = true,
      resize_window = true,
      window_picker = {
        enable = true,
        chars = "asetniol",
        exclude = {
          filetype = { "notify", "packer", "qf", "diff", "fugitive", "fugitiveblame" },
          buftype = { "nofile", "terminal", "help" },
        },
      },
    },
  },
  log = {
    enable = false,
    truncate = false,
    types = {
      all = false,
      config = false,
      copy_paste = false,
      git = false,
      profile = false,
    },
  },
})
-- }}} ⮭

-- 🧿 GitSigns: ⮯ {{{
require('gitsigns').setup {
  signs = {
    add = {
      hl = "GitSignsAdd",
      text = "┣",
      numhl = "GitSignsAddNr",
      linehl = "GitSignsAddLn"
    },
    change = {
      hl = "GitSignsChange",
      text = "┃",
      numhl = "GitSignsChangeNr",
      linehl = "GitSignsChangeLn"
    },
    delete = {
      hl = "GitSignsDelete",
      text = "┫",
      numhl = "GitSignsDeleteNr",
      linehl = "GitSignsDeleteLn"
    },
    topdelete = {
      hl = "GitSignsDelete",
      text = "🠳",
      numhl = "GitSignsDeleteNr",
      linehl = "GitSignsDeleteLn"
    },
    changedelete = {
      hl = "GitSignsChange",
      text = "┃",
      numhl = "GitSignsChangeNr",
      linehl = "GitSignsChangeLn",
    },
  },
  signcolumn = true, -- Toggle with `:Gitsigns toggle_signs`
  numhl = true, -- Toggle with `:Gitsigns toggle_numhl`
  linehl = false, -- Toggle with `:Gitsigns toggle_linehl`
  word_diff = false, -- Toggle with `:Gitsigns toggle_word_diff`
  current_line_blame = false, -- Toggle with `:Gitsigns toggle_current_line_blame`
  watch_gitdir = {
    interval = 1000,
    follow_files = true,
  },
  attach_to_untracked = true,
  current_line_blame_opts = {
    virt_text = true,
    virt_text_pos = "eol", -- 'eol' | 'overlay' | 'right_align'
    delay = 1000,
    ignore_whitespace = false,
  },
  current_line_blame_formatter = '<author>, <author_time:%Y-%m-%d> - <summary>',
  sign_priority = 6,
  update_debounce = 100,
  status_formatter = nil, -- Use default
  max_file_length = 40000,
  preview_config = {
    -- Options passed to nvim_open_win
    border = "rounded",
    style = "minimal",
    relative = "cursor",
    row = 0,
    col = 1,
  },
  yadm = {
    enable = false,
  },
  -- Keymaps
  on_attach = function(bufnr)
    local function map(mode, lhs, rhs, opts)
      opts = vim.tbl_extend("force", { noremap = true, silent = true }, opts or {})
      vim.api.nvim_buf_set_keymap(bufnr, mode, lhs, rhs, opts)
    end

    -- Navigation
    map("n", "]]", "&diff ? ']c' : '<cmd>Gitsigns next_hunk<CR>'", { expr = true })
    map("n", "[[", "&diff ? '[c' : '<cmd>Gitsigns prev_hunk<CR>'", { expr = true })
    -- Actions
    map("n", "<leader>hs", ":Gitsigns stage_hunk<CR>")
    map("v", "<leader>hs", ":Gitsigns stage_hunk<CR>")
    map("n", "<leader>hr", ":Gitsigns reset_hunk<CR>")
    map("v", "<leader>hr", ":Gitsigns reset_hunk<CR>")
    map("n", "<leader>hS", "<cmd>Gitsigns stage_buffer<CR>")
    map("n", "<leader>hu", "<cmd>Gitsigns undo_stage_hunk<CR>")
    map("n", "<leader>hR", "<cmd>Gitsigns reset_buffer<CR>")
    map("n", "<leader>hp", "<cmd>Gitsigns preview_hunk<CR>")
    map("n", "<leader>hb", '<cmd>lua require"gitsigns".blame_line{full=true}<CR>')
    map("n", "<leader>lb", "<cmd>Gitsigns toggle_current_line_blame<CR>")
    map("n", "<leader>hd", "<cmd>Gitsigns toggle_deleted<CR>")
    -- Text object
    map("o", "ih", ":<C-U>Gitsigns select_hunk<CR>")
    map("x", "ih", ":<C-U>Gitsigns select_hunk<CR>")
  end,
}
-- }}} ⮭

-- 🧭 LuaLine: ⮯ {{{
require('lualine').setup {
  options = {
    icons_enabled = true,
    theme = 'catppuccin',
    component_separators = '|',
    section_separators = '',
  },
}
-- }}} ⮭

-- 🔭 Telescope: ⮯ {{{

local actions = require("telescope.actions")

require('telescope').setup({
  defaults = {
    prompt_prefix = "   ",
    selection_caret = "  ",
    path_display = { "smart" },
    mappings = {
      i = {
        ["<C-Down>"] = actions.cycle_history_next,
        ["<C-Up>"] = actions.cycle_history_prev,
        ["<Esc>"] = actions.close,
        ["<C-c>"] = actions.close,
        ["<Down>"] = actions.move_selection_next,
        ["<Up>"] = actions.move_selection_previous,
        ["<PageUp>"] = actions.results_scrolling_up,
        ["<PageDown>"] = actions.results_scrolling_down,
        ["<C-u>"] = actions.preview_scrolling_up,
        ["<C-d>"] = actions.preview_scrolling_down,
        ["<CR>"] = actions.select_default,
        ["<C-h>"] = actions.select_horizontal,
        ["<C-v>"] = actions.select_vertical,
        ["<gt>"] = actions.select_tab,
        ["<Tab>"] = actions.toggle_selection + actions.move_selection_worse,
        ["<S-Tab>"] = actions.toggle_selection + actions.move_selection_better,
        ["<C-f>"] = actions.send_to_qflist + actions.open_qflist,
        ["<C-q>"] = actions.send_selected_to_qflist + actions.open_qflist,
        ["<C-l>"] = actions.complete_tag,
        ["<C-Space>"] = actions.which_key,
      },
    },
  },
  extensions = {
    ["zf-native"] = {
      file = {
        enable = true,
        highlight_results = true,
        match_filename = true,
      },
      generic = {
        enable = true,
        highlight_results = true,
        match_filename = false,
      },
    },
  },
})

require("telescope").load_extension("zf-native")

local map = vim.keymap.set
map('n', '<leader>t<leader>', require('telescope.builtin').find_files)
map('n', '<leader>tt', require('telescope.builtin').live_grep)
map('n', '<leader>tT', require('telescope.builtin').grep_string)
map('n', '<leader>tg', require('telescope.builtin').git_files)
map('n', '<leader>to', require('telescope.builtin').oldfiles)
map('n', '<leader>tb', require('telescope.builtin').buffers)
map('n', '<leader>th', require('telescope.builtin').help_tags)
map('n', '<leader>td', require('telescope.builtin').diagnostics)
map('n', '<leader>tp', require('telescope.builtin').builtin)
map('n', '<leader>tc', require('telescope.builtin').commands)
map('n', '<leader>tl', require('telescope.builtin').loclist)
map('n', '<leader>tq', require('telescope.builtin').quickfix)
map('n', '<leader>tm', require('telescope.builtin').man_pages)
map('n', '<leader>tst', require('telescope.builtin').treesitter)
-- }}} ⮭

-- 🎄 Treesitter: ⮯ {{{
require('nvim-treesitter.configs').setup {
  ensure_installed = {
    "lua", "help", "vim", "c", "cpp", "go", "scss", "yaml", "css", "html",
    "python", "javascript", "json", "typescript", "rust", "bash", "markdown",
    "latex", "regex", "solidity",
  },
  sync_install = true,
  auto_install = true,
  highlight = {
    enable = true,
    disable = function(lang, buf)
      local max_filesize = 100 * 1024 -- 100 KB
      local ok, stats = pcall(vim.loop.fs_stat, vim.api.nvim_buf_get_name(buf))
      if ok and stats and stats.size > max_filesize then
        return true
      end
    end,
    additional_vim_regex_highlighting = false,
  },
  indent = {
    enable = true,
    disable = { 'python' }
  },
  incremental_selection = {
    enable = true,
    keymaps = {
      init_selection = "gn", -- normal: start incremental selection.
      node_incremental = "gn", -- visual: increment to the upper named parent.
      scope_incremental = "gy", -- visual: increment to the upper scope
      node_decremental = "gp", -- visual: decrement to the previous named node.
    },
  },
  context_commentstring = {
    enable = true,
    enable_autocmd = false, -- integration with Comment.nvim
  },
  playground = {
    enable = true,
    disable = {},
    updatetime = 25, -- Debounced time for highlighting nodes in the playground from source code
    persist_queries = false, -- Whether the query persists across vim sessions
    keybindings = {
      toggle_query_editor = "o",
      toggle_hl_groups = "i",
      toggle_injected_languages = "t",
      toggle_anonymous_nodes = "a",
      toggle_language_display = "I",
      focus_language = "f",
      unfocus_language = "F",
      update = "R",
      goto_node = "<CR>",
      show_help = "?",
    },
  },
  autopairs = { enable = true },
}
-- }}} ⮭

-- 🪄 Nvim-Cmp: ⮯ {{{
local cmp = require 'cmp'
local luasnip = require 'luasnip'

cmp.setup {
  snippet = {
    expand = function(args)
      luasnip.lsp_expand(args.body)
    end,
  },
  mapping = cmp.mapping.preset.insert {
    ['<C-d>'] = cmp.mapping.scroll_docs(-4),
    ['<C-f>'] = cmp.mapping.scroll_docs(4),
    ['<C-Space>'] = cmp.mapping.complete(),
    ['<CR>'] = cmp.mapping.confirm {
      behavior = cmp.ConfirmBehavior.Replace,
      select = true,
    },
    ['<Tab>'] = cmp.mapping(function(fallback)
      if cmp.visible() then
        cmp.select_next_item()
      elseif luasnip.expand_or_jumpable() then
        luasnip.expand_or_jump()
      else
        fallback()
      end
    end, { 'i', 's' }),
    ['<S-Tab>'] = cmp.mapping(function(fallback)
      if cmp.visible() then
        cmp.select_prev_item()
      elseif luasnip.jumpable(-1) then
        luasnip.jump(-1)
      else
        fallback()
      end
    end, { 'i', 's' }),
  },
  sources = {
    { name = 'nvim_lsp' },
    { name = 'luasnip' },
  },
}
-- }}} ⮭

-- 📚 LSP Settings: ⮯ {{{
local on_attach = function(_, bufnr)

  local nmap = function(keys, func)
    vim.keymap.set('n', keys, func, { buffer = bufnr })
  end
  nmap('<leader>tsr', require('telescope.builtin').lsp_references)
  nmap('<leader>tss', require('telescope.builtin').lsp_document_symbols)
  nmap('<leader>tsw', require('telescope.builtin').lsp_dynamic_workspace_symbols)
  nmap('<leader>rn', vim.lsp.buf.rename)
  nmap('<leader>ca', vim.lsp.buf.code_action)
  nmap('gd', vim.lsp.buf.definition)
  nmap('gI', vim.lsp.buf.implementation)
  nmap('<leader>D', vim.lsp.buf.type_definition)
  nmap('K', vim.lsp.buf.hover)
  nmap('<C-k>', vim.lsp.buf.signature_help)
end

local servers = {
  -- clangd = {},
  -- gopls = {},
  -- pyright = {},
  -- rust_analyzer = {},
  -- tsserver = {},

  sumneko_lua = {
    Lua = {
      workspace = { checkThirdParty = false },
      telemetry = { enable = false },
    },
  },
}

-- Ensure the servers above are installed
local mason_lspconfig = require 'mason-lspconfig'
local capabilities = vim.lsp.protocol.make_client_capabilities()
capabilities = require('cmp_nvim_lsp').default_capabilities(capabilities)

mason_lspconfig.setup {
  ensure_installed = vim.tbl_keys(servers),
}

mason_lspconfig.setup_handlers {
  function(server_name)
    require('lspconfig')[server_name].setup {
      capabilities = capabilities,
      on_attach = on_attach,
      settings = servers[server_name],
    }
  end,
}
-- }}} ⮭

-- 🍐 Nvim-Autopairs: ⮯ {{{
local npairs = require("nvim-autopairs")
local ts_conds = require('nvim-autopairs.ts-conds')
local Rule = require('nvim-autopairs.rule')
local cmp_autopairs = require("nvim-autopairs.completion.cmp")

npairs.setup({
  enable_check_bracket_line = false,
  fast_wrap = {
    map = "<C-e>",
    chars = { "{", "[", "(", '"', "'", "`", "<" },
    pattern = string.gsub([[ [%'%"%)%>%]%)%}%,] ]], "%s+", ""),
    offset = -1,
    end_key = "$",
    keys = "asetniol",
    check_comma = true,
    highlight = "Search",
    highlight_grey = "Comment",
  },
  check_ts = true,
  ts_config = {
    lua = { 'string' }, -- it will not add a pair on that treesitter node
    javascript = { 'template_string' },
    java = false, -- don't check treesitter on java
  }
})

-- press % => %% only while inside a comment or string
npairs.add_rules({
  Rule("%", "%", "lua")
      :with_pair(ts_conds.is_ts_node({ 'string', 'comment' })),
  Rule("$", "$", "lua")
      :with_pair(ts_conds.is_not_ts_node({ 'function' }))
})

cmp.event:on(
  "confirm_done",
  cmp_autopairs.on_confirm_done({
    map_char = { tex = "" },
  })
)
-- }}} ⮭
