return {
	settings = {
		python = {
			analysis = {
				typeCheckingMode = "standard",
				autoSearchPaths = true,
				useLibraryCodeForTypes = true,
				diagnosticSeverityOverrides = {
					reportUnknownMemberType = "none",
					reportUnknownArgumentType = "none",
					reportUnknownVariableType = "none",
					reportMissingTypeStubs = "none",
				},
			},
		},
	},
}
