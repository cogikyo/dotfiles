/**
 * @name BetterCodeblocks
 * @invite undefined
 * @authorLink undefined
 * @donate undefined
 * @patreon undefined
 * @website https://github.com/vBread/bd-contributions/tree/master/BetterCodeblocks
 * @source https://github.com/vBread/bd-contributions/blob/master/BetterCodeblocks/BetterCodeblocks.plugin.js
 */
/*@cc_on
@if (@_jscript)
	
	// Offer to self-install for clueless users that try to run this directly.
	var shell = WScript.CreateObject("WScript.Shell");
	var fs = new ActiveXObject("Scripting.FileSystemObject");
	var pathPlugins = shell.ExpandEnvironmentStrings("%APPDATA%\BetterDiscord\plugins");
	var pathSelf = WScript.ScriptFullName;
	// Put the user at ease by addressing them in the first person
	shell.Popup("It looks like you've mistakenly tried to run me directly. \n(Don't do that!)", 0, "I'm a plugin for BetterDiscord", 0x30);
	if (fs.GetParentFolderName(pathSelf) === fs.GetAbsolutePathName(pathPlugins)) {
		shell.Popup("I'm in the correct folder already.", 0, "I'm already installed", 0x40);
	} else if (!fs.FolderExists(pathPlugins)) {
		shell.Popup("I can't find the BetterDiscord plugins folder.\nAre you sure it's even installed?", 0, "Can't install myself", 0x10);
	} else if (shell.Popup("Should I copy myself to BetterDiscord's plugins folder for you?", 0, "Do you need some help?", 0x34) === 6) {
		fs.CopyFile(pathSelf, fs.BuildPath(pathPlugins, fs.GetFileName(pathSelf)), true);
		// Show the user where to put plugins in the future
		shell.Exec("explorer " + pathPlugins);
		shell.Popup("I'm installed!", 0, "Successfully installed", 0x40);
	}
	WScript.Quit();

@else@*/

module.exports = (() => {
    const config = {"main":"index.js","info":{"name":"BetterCodeblocks","authors":[{"name":"Bread","discord_id":"304260051915374603","github_username":"vBread"}],"version":"1.3.3","description":"Enhances the look and feel of Discord's codeblocks with customizable colors","github":"https://github.com/vBread/bd-contributions/tree/master/BetterCodeblocks","github_raw":"https://github.com/vBread/bd-contributions/blob/master/BetterCodeblocks/BetterCodeblocks.plugin.js"},"changelog":[{"title":"Fixes","type":"fixed","items":["Improve caching performance", "Fixed a vulnerability issue."]}]};

    return !global.ZeresPluginLibrary ? class {
        constructor() {this._config = config;}
        getName() {return config.info.name;}
        getAuthor() {return config.info.authors.map(a => a.name).join(", ");}
        getDescription() {return config.info.description;}
        getVersion() {return config.info.version;}
        load() {
            BdApi.showConfirmationModal("Library Missing", `The library plugin needed for ${config.info.name} is missing. Please click Download Now to install it.`, {
                confirmText: "Download Now",
                cancelText: "Cancel",
                onConfirm: () => {
                    require("request").get("https://rauenzi.github.io/BDPluginLibrary/release/0PluginLibrary.plugin.js", async (error, response, body) => {
                        if (error) return require("electron").shell.openExternal("https://betterdiscord.net/ghdl?url=https://raw.githubusercontent.com/rauenzi/BDPluginLibrary/master/release/0PluginLibrary.plugin.js");
                        await new Promise(r => require("fs").writeFile(require("path").join(BdApi.Plugins.folder, "0PluginLibrary.plugin.js"), body, r));
                    });
                }
            });
        }
        start() {}
        stop() {}
    } : (([Plugin, Api]) => {
        const plugin = (Plugin, Library) => {

	const https = require('https')
	const path = require('path');

	const { Patcher, WebpackModules, DiscordModules, PluginUtilities, Settings } = Library;
	const { SettingGroup, Textbox } = Settings
	const { React } = DiscordModules

	const { Messages } = WebpackModules.getByProps('Messages')
	const PLUGIN_ID = 'BetterCodeblocks'

	const defaults = {
		hljs: {
			additions: '#859900',
			attributeSelectors: '#cb4b16',
			attributes: '#b58900',
			background: '#282c34',
			bold: null,
			builtIns: '#dc322f',
			bulletPoints: '#cb4b16',
			classSelectors: '#268bd2',
			classes: '#b58900',
			codeblocks: null,
			comments: '#586e75',
			deletions: '#dc322f',
			documentationTags: '#2aa198',
			foreground: '#839496',
			headings: '#268bd2',
			hyperlinks: '#cb4b16',
			idSelectors: '#268bd2',
			italics: null,
			keywords: '#859900',
			lineNumbers: null,
			literals: '#2aa198',
			meta: '#cb4b16',
			metaStrings: '#2aa198',
			metaKeywords: '#cb4b16',
			names: '#268bd2',
			numbers: '#2aa198',
			operators: null,
			parameters: null,
			pseudoSelectors: '#cb4b16',
			punctuation: null,
			quotes: '#586e75',
			regularExpressions: '#2aa198',
			sections: '#268bd2',
			stringInterpolation: '#cb4b16',
			strings: '#2aa198',
			symbols: '#cb4b16',
			tags: null,
			tagSelectors: '#859900',
			titles: '#268bd2',
			types: '#b58900',
			unspecifiedAttributes: '#b58900',
			variables: '#b58900',
		},
		theme: ''
	}

	let settings = PluginUtilities.loadSettings(PLUGIN_ID, defaults)

	return class BetterCodeblocks extends Plugin {
		onStart() {
			const parser = WebpackModules.getByProps('parse', 'parseTopic')

			this.unpatch = Patcher.after(parser.defaultRules.codeBlock, 'react', (_, nodes, output) => {
				this.inject(nodes, output)

				nodes[0].content = this.dedent(nodes[0].content)

				return output
			});

			PluginUtilities.addStyle(PLUGIN_ID, this.css)
		}

		onStop() {
			PluginUtilities.removeStyle(PLUGIN_ID)
			this.unpatch()
		}

		getSettingsPanel() {
			const panel = document.createElement("form");
			panel.classList.add("form");
			panel.style.setProperty("width", "100%");

			new SettingGroup('Customization').appendTo(panel).append(
				new SettingGroup('Main').append(
					...this.createColorInput([
						['Foreground', 'Applies to any text without a style'],
						['Background', 'Applies to the background of the codeblock'],
						['Line Numbers', 'Applies to the line numbers']
					])
				),

				new SettingGroup('General').append(
					...this.createColorInput([
						['Keywords', 'Applies to keywords in a regular ALGOL-style language'],
						['Built-Ins', 'Applies to built-in objects and/or library objects'],
						['Types', 'Applies to syntactically significant types'],
						['Literals', 'Applies to special identifiers for built-in values'],
						['Numbers', 'Applies to numbers, including units and modifiers (Not widely supported)'],
						['Operators', 'Applies to logical and mathematical operators'],
						['Punctuation', 'Applies to auxillary punctuation (Not widely supported)'],
						['Regular Expressions', 'Applies to literal regular expressions'],
						['Strings', 'Applies to literal strings/characters'],
						['String Interpolation', 'Applies to parsed sections inside a literal string'],
						['Symbols', 'Applies to symbolic constants and interned strings'],
						['Classes', 'Applies to class-level declarations'],
						['Functions', 'Applies to function or method declarations'],
						['Variables', 'Applies to variables declarations'],
						['Titles', 'Applies to other declarations'],
						['Parameters', 'Applies to function arguments at the place of declaration'],
						['Comments', 'Applies to line and block comments'],
						['Documentation Tags', 'Applies to documentation markup within comments']
					])
				),

				new SettingGroup('Meta').append(
					...this.createColorInput([
						['Meta', 'Applies to modifiers, annotations, preprocessor directives, etc.'],
						['Meta Keywords', 'Applies to keywords or built-ins within meta constructs'],
						['Meta Strings', 'Applies to strings within meta constructs']
					]),
				),

				new SettingGroup('Markdown').append(
					...this.createColorInput([
						['Bullet Points', 'Applies to bullet points in an unordered list'],
						['Codeblocks', 'Applies to code blocks'],
						['Italics', 'Applies to italicized text'],
						['Bold', 'Applies to bold text'],
						['Hyperlinks', 'Applies to hyperlinks'],
						['Quotes', 'Applies to quotations or blockquotes'],
						['Headings', 'Applies to headings']
					])
				),

				new SettingGroup('CSS').append(
					...this.createColorInput([
						['Tag Selectors', 'Applies to tag selectors'],
						['ID Selectors', 'Applies to ID selectors'],
						['Class Selectors', 'Applies to class selectors'],
						['Attribute Selectors', 'Applies to attribute selectors'],
						['Pseudo Selectors', 'Applies to pseudo selectors']
					])
				),

				new SettingGroup('Misc').append(
					...this.createColorInput([
						['Sections', 'Applies to headings of a section in a config file'],
						['Tags', 'Applies to XML and HTML tags'],
						['Names', 'Applies to XML and HTML tag names and S-expressions'],
						['Unspecified Attributes', 'Applies to names of an attribute with no language defined semantics and sub-attribute within another highlighted object'],
						['Attributes', 'Applies to names of an attribute followed by a structured value'],
						['Additions', 'Applies to diff additions'],
						['Deletions', 'Applies to diff deletions']
					])
				)
			)

			new SettingGroup('Themes').appendTo(panel).append(
				new Textbox(
					'Custom Theme',
					'Paste a link to a raw JSON file.',
					settings.theme,
					this.readURL.bind(this)
				)
			)

			const div = document.createElement('div')
			div.innerHTML = '<a href="https://github.com/vBread/bd-contributions/blob/master/BetterCodeblocks/UPLOADING.md">Uploading Guide</a>'

			panel.appendChild(div)

			return panel
		}

		createColorInput(data) {
			const pickers = []

			for (const [title, description] of data) {
				let key = title.split(/ |-/);
				key = key[0].toLowerCase() + (key[1] || '');

				const defaultValue = defaults.hljs[key] || defaults.hljs.foreground
				const value = settings.hljs[key]

				console.log(defaultValue, value)

				pickers.push(new Textbox(
					title,
					`${description}. Default: ${defaultValue}`,
					value && Object.keys(value).length ? value : defaultValue,
					(color) => this.updateColor(key, color)
				))
			}

			return pickers
		}

		updateColor(key, color) {
			settings.hljs[key] = color
			this.save()
		}

		readURL(value) {
			if (!value) {
				settings = defaults
				return this.save()
			};

			if (typeof value === 'string') {
				if (!/json|(s[ca]|le|c)ss$/.test(value)) return;

				https.get(value, this.parseResponse.bind(this, value))
			}
		}

		parseResponse(url, response) {
			response.setEncoding('utf8').on('data', (chunk) => {
				switch (path.extname(url)) {
					case '.json': {
						const valid = Object.fromEntries(
							Object.entries(JSON.parse(chunk)).filter(([key]) => key in defaults.hljs)
						)

						settings = {
							...settings,
							hljs: valid,
							theme: url
						}

						break;
					}
				}

				this.save()
			})
		}

		save() {
			PluginUtilities.saveSettings(PLUGIN_ID, settings)
			this.reload()
		}

		reload() {
			PluginUtilities.removeStyle(PLUGIN_ID)
			PluginUtilities.addStyle(PLUGIN_ID, this.css)
		}


		dedent(content) {
			content = content.replace(/\t/g, ' '.repeat(4));

			const min = content.match(/^[^\S\n]+/gm)?.reduce((x, y) => Math.min(x, y.length), Infinity) ?? 0;

			return !min ? content : content.replace(new RegExp(`^ {${min}}`, "gm"), '');
		}

		inject(nodes, output) {
			const render = output.props.render;

			output.props.render = (properties) => {
				const codeblock = render(properties);
				const { props } = codeblock.props.children;

				const classes = props.className.split(' ');
				const language = nodes ? nodes[0].lang : classes[classes.indexOf('hljs') + 1];

				const innerHTML = props.dangerouslySetInnerHTML
				let lines;

				if (innerHTML) {
					lines = innerHTML.__html.replace(/<span class="(hljs-[a-z]+)">([^<]*)<\/span>/g, (_, className, code) => code
						.split('\n')
						.map((line) => `<span class='${className}'>${line}</span>`)
						.join('\n')
					).split('\n')
				} else {
					lines = props.children.split('\n')
				}

				const dangeorus = Boolean(props.dangerouslySetInnerHTML);
				delete props.dangerouslySetInnerHTML;

				props.children = this.render(language, lines, dangeorus);

				return codeblock;
			};
		}

		render(language, lines, dangerous) {
			const $hljs = DiscordModules.hljs

			if ($hljs && typeof $hljs.getLanguage === 'function') {
				language = $hljs.getLanguage(language);
			}

			return React.createElement(React.Fragment, null,
				language && React.createElement('div', { className: 'bd-codeblock-language' }, language.name),

				React.createElement('table', { className: 'bd-codeblock-table' },
					...lines.map((line, i) => React.createElement('tr', null,
						React.createElement('td', null, i + 1),
						React.createElement('td',
							language && dangerous ? {
								dangerouslySetInnerHTML: {
									__html: line
								}
							} : {
								children: line
							}
						)
					))
				),

				React.createElement('button', {
					className: 'bd-codeblock-copy-btn',
					onClick: this.clickHandler
				}, Messages.COPY)
			)
		}

		clickHandler({ target }) {
			const { clipboard } = require('electron')

			if (target.classList.contains('copied')) return

			target.innerText = Messages.ACCOUNT_USERNAME_COPY_SUCCESS_1;
			target.classList.add('copied');

			setTimeout(() => {
				target.innerText = Messages.COPY;
				target.classList.remove('copied');
			}, 1e3);

			const code = [...target.parentElement.querySelectorAll('td:last-child')]
				.map((cell) => cell.textContent).join('\n');

			clipboard.writeText(code);
		}

		get css() {
			return `
				.hljs {
					background-color: ${settings.hljs.background} !important;
					color: ${settings.hljs.foreground};
					position: relative;
				}

				.hljs:not([class$='hljs']) {
					padding-top: 2px;
				}

				.bd-codeblock-language {
					color: var(--text-normal);
					border-bottom: 1px solid var(--background-modifier-accent);
					padding: 0 5px;
					margin-bottom: 6px;
					font-size: .8em;
					font-family: 'Raleway', sans-serif;
					font-weight: bold;
				}

				.bd-codeblock-table {
					border-collapse: collapse;
				}

				.bd-codeblock-table tr {
					height: 19px;
					width: 100%;
				}

				.bd-codeblock-table td:first-child {
					border-right: 1px solid var(--background-modifier-accent);
					padding-left: 5px;
					padding-right: 8px;
					user-select: none;
					color: ${settings.hljs.lineNumbers}
				}

				.bd-codeblock-table td:last-child {
					padding-left: 8px;
					word-break: break-all;
				}

				.bd-codeblock-copy-btn {
					color: var(--text-normal);
					border-radius: 4px;
					line-height: 20px;
					padding: 0 10px;
					font-family: 'Raleway', sans-serif;
					font-size: .8em;
					text-transform: uppercase;
					font-weight: bold;
					margin: 3px;
					background: var(--background-floating);
					position: absolute;
					right: 0 !important;
					bottom: 0 !important;
					opacity: 0;
					transition: .3s;
				}

				.bd-codeblock-copy-btn.copied {
					background-color: #43b581;
					opacity: 1;
				}

				.hljs:hover .bd-codeblock-copy-btn {
					opacity: 1;
				}

				${this.codeBlockStyle}				

				.codeLine-14BKbG > span > span {
					color: ${settings.hljs.foreground};
				}

				${this.textBoxStyle}
			`
		}

		get codeBlockStyle() {
			return `
				.hljs > .bd-codeblock-table > tr > td > span > .hljs-tag {
					color: ${settings.hljs.tags};
				}

				.hljs > .bd-codeblock-table > tr > td > span > .hljs-tag > .hljs-name {
					color: ${settings.hljs.names};
				}

				.hljs > .bd-codeblock-table > tr > td > span > .hljs-tag > .hljs-attr {
					color: ${settings.hljs.unspecifiedAttributes};
				}

				.hljs > .bd-codeblock-table > tr > td > .bash > .hljs-built_in {
					color: ${settings.hljs.builtIns};
				}

				.hljs > .bd-codeblock-table > tr > td > .bash > .hljs-variable {
					color: ${settings.hljs.variables};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-tag {
					color: ${settings.hljs.tags} !important;
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-tag > .hljs-name {
					color: ${settings.hljs.names};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-tag > .hljs-attr {
					color: ${settings.hljs.attributes};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-function > .hljs-params {
					color: ${settings.hljs.parameters};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-function > .hljs-type {
					color: ${settings.hljs.types};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-function > .hljs-params > .hljs-type {
					color: ${settings.hljs.types};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-operator {
					color: ${settings.hljs.operators};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-punctuation {
					color: ${settings.hljs.punctuation};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-name {
					color: ${settings.hljs.names};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-params {
					color: ${settings.hljs.parameters};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-title {
					color: ${settings.hljs.title};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-function > .hljs-params > .hljs-built_in {
					color: ${settings.hljs.builtIns};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-selector-attr {
					color: ${settings.hljs.attributeSelectors};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-type {
					color: ${settings.hljs.types};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-selector-id {
					color: ${settings.hljs.idSelectors};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-selector-pseudo {
					color: ${settings.hljs.pseudoSelectors};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-bullet {
					color: ${settings.hljs.bulletPoints};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-emphasis {
					color: ${settings.hljs.italics}
				}

				.hljs.md > .bd-codeblock-table > tr > td > .hljs-section {
					color: ${settings.hljs.headings}
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-link {
					color: ${settings.hljs.hyperlinks}
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-strong {
					color: ${settings.hljs.bold}
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-addition {
					color: ${settings.hljs.additions};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-deletion {
					color: ${settings.hljs.deletions};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-regexp {
					color: ${settings.hljs.regularExpressions};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-doctag {
					color: ${settings.hljs.documentationTags};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-built_in {
					color: ${settings.hljs.builtIns};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-attr {
					color: ${settings.hljs.attributes};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-section {
					color: ${settings.hljs.sections};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-meta {
					color: ${settings.hljs.meta};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-literal {
					color: ${settings.hljs.literals};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-function .hljs-title {
					color: ${settings.hljs.functions};
				}

				.hljs > .bd-codeblock-table > tr > td > .hljs-class .hljs-title {
					color: ${settings.hljs.classes};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-keyword {
					color: ${settings.hljs.keywords};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-selector-tag {
					color: ${settings.hljs.tagSelectors};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-selector-class {
					color: ${settings.hljs.classSelectors};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-attribute {
					color: ${settings.hljs.attributes};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-symbol {
					color: ${settings.hljs.symbols};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-number {
					color: ${settings.hljs.numbers};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-string {
					color: ${settings.hljs.strings};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-string > .hljs-subst > .hljs-built_in {
					color: ${settings.hljs.builtIns};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-subst {
					color: ${settings.hljs.stringInterpolation};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-code {
					color: ${settings.hljs.codeblocks};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-comment {
					color: ${settings.hljs.comments};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-quote {
					color: ${settings.hljs.quotes};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-variable {
					color: ${settings.hljs.variables};
				}

				.hljs > .bd-codeblock-table > tr > td .hljs-meta-string {
					color: ${settings.hljs.metaStrings};
				}
			`
		}

		get textBoxStyle() {
			return this.codeBlockStyle.replace(/\.hljs[\w\. ->]+td/gm, '.codeLine-14BKbG > span > span')
		}
	};
};
        return plugin(Plugin, Api);
    })(global.ZeresPluginLibrary.buildPlugin(config));
})();
/*@end@*/