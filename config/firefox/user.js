// Startup behavior.
user_pref("browser.startup.page", 1);
user_pref("browser.sessionstore.resume_from_crash", false);

// Browser chrome and repo-managed userChrome.css.
user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", true);
user_pref("layout.css.devPixelsPerPx", "1.1");
user_pref("browser.toolbars.bookmarks.visibility", "never");

// Desktop entries route external URLs through `hyprd browser open`.
// hyprd focuses or swaps the workspace-local Firefox first, then Firefox remoting opens a tab there.
user_pref("browser.link.open_newwindow", 3);
user_pref("browser.link.open_newwindow.override.external", 3);
user_pref("browser.link.open_newwindow.restriction", 0);

// Built-in surfaces that are replaced by local workflows.
user_pref("pdfjs.sidebarViewOnLoad", 0);
user_pref("extensions.pocket.enabled", false);
user_pref("browser.newtabpage.enabled", false);

// Disalbe tab previews
user_pref("browser.tabs.hoverPreview.enabled", false);
user_pref("browser.tabs.cardPreview.enabled", false);
user_pref("browser.tabs.groups.hoverPreview.enabled", false);

// New tab recommendations and sponsored content.
user_pref("browser.discovery.enabled", false);
user_pref("browser.newtabpage.activity-stream.feeds.topsites", false);
user_pref("browser.newtabpage.activity-stream.feeds.section.topstories", false);
user_pref("browser.newtabpage.activity-stream.showSponsored", false);
user_pref("browser.newtabpage.activity-stream.showSponsoredTopSites", false);

// Lightweight privacy defaults.
user_pref("privacy.trackingprotection.enabled", true);
user_pref("privacy.trackingprotection.socialtracking.enabled", true);
user_pref("privacy.donottrackheader.enabled", true);

// Input feel.
user_pref("general.smoothScroll", true);
