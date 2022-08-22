/**
 * @name HideChannels
 * @author Farcrada
 * @version 2.2.2
 * @description Hide channel list from view.
 *
 * @invite qH6UWCwfTu
 * @website https://github.com/Farcrada/DiscordPlugins
 * @source https://github.com/Farcrada/DiscordPlugins/edit/master/Hide-Channels/HideChannels.plugin.js
 * @updateUrl https://raw.githubusercontent.com/Farcrada/DiscordPlugins/master/Hide-Channels/HideChannels.plugin.js
 */

/** @type {typeof import("react")} */
const React = BdApi.React;

const config = {
	info: {
		name: "Hide Channels",
		id: "HideChannels",
		description: "Hide channel list from view.",
		version: "2.2.2",
		author: "Farcrada",
		updateUrl: "https://raw.githubusercontent.com/Farcrada/DiscordPlugins/master/Hide-Channels/HideChannels.plugin.js"
	},
	constants: {
		//The names we need for CSS
		cssStyle: "HideChannelsStyle",
		hideElementsName: "hideChannelElement",
		buttonID: "toggleChannels",
		buttonHidden: "channelsHidden",
		buttonVisible: "channelsVisible"
	}
}


module.exports = class HideChannels {
	//I like my spaces.
	getName() { return config.info.name; }


	load() {
		try { global.ZeresPluginLibrary.PluginUpdater.checkForUpdate(config.info.name, config.info.version, config.info.updateUrl); }
		catch (err) { console.error(this.getName(), "Failed to reach the ZeresPluginLibrary for Plugin Updater.", err); }
	}

	start() {
		try {
			//React components for settings
			this.WindowInfoStore = BdApi.findModuleByProps("isFocused", "isElementFullScreen");
			this.KeybindStore = BdApi.findModuleByProps("toCombo");

			//The sidebar to "minimize"/hide
			this.sidebarClass = BdApi.findModuleByProps("container", "base").sidebar;

			//And the keybind
			this.keybindSetting = this.checkKeybindLoad(BdApi.loadData(config.info.id, "keybind"));
			this.keybind = this.keybindSetting.split('+');

			//Predefine for the eventlistener
			this.currentlyPressed = {};

			//Check if there is any CSS we have already, and remove it.
			BdApi.clearCSS(config.constants.cssStyle);

			//Now inject our (new) CSS
			BdApi.injectCSS(config.constants.cssStyle, `
/* Button CSS */
#${config.constants.buttonID} {
    min-width: 24px;
    height: 24px;
    background-position: center !important;
    background-size: 100% !important;
    opacity: 0.8;
    cursor: pointer;
}

/* How the button looks */
.theme-dark #${config.constants.buttonID}.${config.constants.buttonVisible} {
    background: url(data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0iI2ZmZiIgd2lkdGg9IjE4cHgiIGhlaWdodD0iMThweCI+PHBhdGggZD0iTTE4LjQxIDE2LjU5TDEzLjgyIDEybDQuNTktNC41OUwxNyA2bC02IDYgNiA2ek02IDZoMnYxMkg2eiIvPjxwYXRoIGQ9Ik0yNCAyNEgwVjBoMjR2MjR6IiBmaWxsPSJub25lIi8+PC9zdmc+) no-repeat;
}
.theme-dark #${config.constants.buttonID}.${config.constants.buttonHidden} {
    background: url(data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0iI2ZmZiIgd2lkdGg9IjE4cHgiIGhlaWdodD0iMThweCI+PHBhdGggZD0iTTAgMGgyNHYyNEgwVjB6IiBmaWxsPSJub25lIi8+PHBhdGggZD0iTTUuNTkgNy40MUwxMC4xOCAxMmwtNC41OSA0LjU5TDcgMThsNi02LTYtNnpNMTYgNmgydjEyaC0yeiIvPjwvc3ZnPg==) no-repeat;
}
/* In light theme */
.theme-light #${config.constants.buttonID}.${config.constants.buttonVisible} {
    background: url(data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0iIzRmNTY2MCIgd2lkdGg9IjE4cHgiIGhlaWdodD0iMThweCI+PHBhdGggZD0iTTE4LjQxIDE2LjU5TDEzLjgyIDEybDQuNTktNC41OUwxNyA2bC02IDYgNiA2ek02IDZoMnYxMkg2eiIvPjxwYXRoIGQ9Ik0yNCAyNEgwVjBoMjR2MjR6IiBmaWxsPSJub25lIi8+PC9zdmc+) no-repeat;
}
.theme-light #${config.constants.buttonID}.${config.constants.buttonHidden} {
    background: url(data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0iIzRmNTY2MCIgd2lkdGg9IjE4cHgiIGhlaWdodD0iMThweCI+PHBhdGggZD0iTTAgMGgyNHYyNEgwVjB6IiBmaWxsPSJub25lIi8+PHBhdGggZD0iTTUuNTkgNy40MUwxMC4xOCAxMmwtNC41OSA0LjU5TDcgMThsNi02LTYtNnpNMTYgNmgydjEyaC0yeiIvPjwvc3ZnPg==) no-repeat;
}

/* Attached CSS to sidebar */
.${config.constants.hideElementsName} {
    width: 0 !important;
}

/* Set animations */
.${this.sidebarClass} {
    transition: width 400ms ease;
}`);

			//Render the button and we're off to the races!
			this.patchTitleBar();
		}
		catch (err) {
			try {
				console.error("Attempting to stop after starting error...", err)
				this.stop();
			}
			catch (err) {
				console.error(this.getName() + ".stop()", err);
			}
		}
	}

	getSettingsPanel() {
		//Settings window is lazy loaded so we need to cache this after it's been loaded (i.e. open settings).
		//This also allows for a (delayed) call to retrieve a way to prompt a Form
		if (!this.KeybindRecorder) {
			this.KeybindRecorder = BdApi.findModuleByDisplayName("KeybindRecorder");
			this.FormItem = BdApi.findModuleByProps("FormItem").FormItem;
		}

		//Return our keybind settings wrapped in a form item
		return React.createElement(this.FormItem, {
			title: "Toggle by keybind:"
		},
			//Containing a keybind recorder.
			React.createElement(this.KeybindRecorder, {
				//The `keyup` and `keydown` events register the Ctrl key different
				//We need to accomodate for that
				defaultValue: this.KeybindStore.toCombo(this.keybindSetting.replace("control", "ctrl")),
				onChange: (e) => {
					//Convert the keybind to current locale
					//Once again accomodate for event differences
					const keybindString = this.KeybindStore.toString(e).toLowerCase().replace("ctrl", "control");

					//Set the keybind and save it.
					BdApi.saveData(config.info.id, "keybind", keybindString);
					//And the keybindSetting
					this.keybindSetting = keybindString;
					this.keybind = keybindString.split('+');
				}
			}));
	}

	stop() {
		BdApi.Patcher.unpatchAll(config.info.id);

		//Our CSS
		BdApi.clearCSS(config.constants.cssStyle);

		//And if there are remnants of css left,
		//make sure we remove the class from the sidebar to ensure visual confirmation.
		let sidebar = document.querySelector(`.${this.sidebarClass}`);
		if (sidebar?.classList.contains(config.constants.hideElementsName))
			sidebar.classList.remove(config.constants.hideElementsName);
	}

	patchTitleBar() {
		//The header bar above the "chat"; this is the same for the `Split View`.
		const HeaderBar = BdApi.findModule(m => m?.default?.displayName === "HeaderBar");

		BdApi.Patcher.before(config.info.id, HeaderBar, "default", (thisObject, methodArguments, returnValue) => {
			//When elements are being re-rendered we need to check if there actually is a place for us.
			//Along with that we need to check if what we're adding to is an array;
			//because if not we'll render a button on the split view.

			//Also: Prevent thread button appearing with this first line.
			if (Array.isArray(methodArguments[0]?.children))
				//Make sure we're on the "original" headerbar and not that of a Voice channel's chat.
				if (methodArguments[0].children.some?.(child => child?.type?.displayName === "HeaderGuildBreadcrumb" || child?.type?.displayName === "Title"))
					//Make sure our component isn't already present.
					if (!methodArguments[0].children.some?.(child => child?.key === config.info.id))
						//And since we want to be on the most left of the header bar for style we unshift into the array.
						methodArguments[0].children.unshift?.(React.createElement(this.hideChannelComponent, { key: config.info.id }));
		});
	}

	/**
	 * React component for our button.
	 * @returns React element
	 */
	hideChannelComponent = () => {
		//Only fetch the sidebar on a rerender.
		const sidebarNode = document.querySelector(`.${this.sidebarClass}`),
			//When a state updates, it rerenders.
			[hidden, setHidden] = React.useState(
				//Check on a rerender where our side bar is so we can correctly reflect this.
				sidebarNode?.classList.contains(config.constants.hideElementsName));

		/**
		 * Use this to make a despensable easy to use listener with React.
		 * @param {string} eventName The name of the event to listen for.
		 * @param {callback} callback Function to call when said event is triggered.
		 * @param {boolean} bubbling Handle bubbling or not
		 * @param {object} [target] The object to attach our listener to.
		 */
		function useListener(eventName, callback, bubbling, target = document) {
			React.useEffect(() => {
				//ComponentDidMount
				target.addEventListener(eventName, callback, bubbling);
				//ComponentWillUnmount
				return () => target.removeEventListener(eventName, callback, bubbling);
			});
		}

		function useWindowChangeListener(windowStore, callback) {
			React.useEffect(() => {
				windowStore.addChangeListener(callback);
				return () => windowStore.removeChangeListener(callback);
			});
		}

		/**
		 * Adds and removes our CSS to make our sidebar appear and disappear.
		 * @param {Node} sidebar Sidebar node we want to toggle.
		 * @returns The passed state in reverse.
		 */
		function toggleSidebar(sidebar) {

			/**
			 * @param {boolean} state State that determines the toggle.
			 */
			return state => {
				//If it is showing, we need to hide it.
				if (!state)
					//We hide it through CSS by adding a class.
					sidebar?.classList.add(config.constants.hideElementsName);
				//If it is hidden, we need to show it.
				else
					sidebar?.classList.remove(config.constants.hideElementsName);

				return !state;
			};
		}

		//Keydown event
		useListener("keydown", e => {
			//Since we made this an object,
			//we can make new propertire with `[]`
			this.currentlyPressed[e.key.toLowerCase()] = true;

			//Account for bubbling and attach to the global: `window`
		}, true, window);

		//Keyup event
		useListener("keyup", e => {
			//Check if every currentlyPessed is in our saved keybind.
			if (this.keybind.every(key => this.currentlyPressed[key.toLowerCase()] === true))
				//Toggle the sidebar and rerender on toggle; change the state
				setHidden(toggleSidebar(sidebarNode));

			//Current key goes up, so...
			this.currentlyPressed[e.key.toLowerCase()] = false;

			//Account for bubbling and attach to the global: `window`
		}, true, window);

		//Lose focus event
		useWindowChangeListener(this.WindowInfoStore, () => {
			//Clear when it gets back into focus
			if (this.WindowInfoStore.isFocused())
				this.currentlyPressed = {};
		});

		//Return our element.
		return React.createElement("div", {
			//Styling
			id: config.constants.buttonID,
			//To identify our object
			key: "hideChannelComponent",
			//The icon
			className: hidden ? config.constants.buttonHidden : config.constants.buttonVisible,
			//Toggle the sidebar and rerender on toggle; change the state.
			onClick: () => setHidden(toggleSidebar(sidebarNode))
		});
	}

	/**
	 * Checks the given keybind for validity. If not valid returns a default keybind.
	 * @param {String|Array.<number>|Array.<Array.<number>>} keybindToLoad The keybind to filter and load in.
	 * @param {String} [defaultKeybind] A default keybind to fall back on in case of invalidity.
	 * @returns Will return the keybind or return a default keybind.
	 */
	checkKeybindLoad(keybindToLoad, defaultKeybind = "control+h") {
		defaultKeybind = defaultKeybind.toLowerCase().replace("ctrl", "control");

		//If no keybind
		if (!keybindToLoad)
			return defaultKeybind;

		//Error sensitive, so just plump it into a try-catch
		try {
			//If it's already a string, double check it
			if (typeof (keybindToLoad) === typeof (defaultKeybind)) {
				keybindToLoad = keybindToLoad.toLowerCase().replace("control", "ctrl");
				//Does it go into a combo? (i.e.: is it the correct format?)
				if (this.KeybindStore.toCombo(keybindToLoad))
					return keybindToLoad.replace("ctrl", "control");
				else
					return defaultKeybind;
			}
			else
				//If it's not a string, check if it's a combo.
				if (this.KeybindStore.toString(keybindToLoad))
					return this.KeybindStore.toString(keybindToLoad).toLowerCase().replace("ctrl", "control");
		}
		catch (e) { return defaultKeybind; }
	}
}
