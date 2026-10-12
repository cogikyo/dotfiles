export interface ClaudeCredentials {
    accessToken: string;
    refreshToken: string;
    expiresAt: number;
    subscriptionType?: string;
}
export interface ClaudeAccount {
    label: string;
    source: string;
    configDir?: string;
    credentials: ClaudeCredentials;
}
export declare const PRIMARY_SERVICE = "Claude Code-credentials";
export declare function parseCredentials(raw: string): ClaudeCredentials | null;
export declare function readCredentialsFile(configDir?: string): ClaudeCredentials | null;
export declare function keychainSuffixForDir(dir: string): string;
export declare function clearSuffixToDirCache(): void;
export declare function buildAccountLabels(credsList: ClaudeCredentials[], emails?: (string | null)[], sources?: (string | null)[]): string[];
export declare function readAllClaudeAccounts(): ClaudeAccount[];
export declare function updateCredentialBlob(existingJson: string, newCreds: {
    accessToken: string;
    refreshToken: string;
    expiresAt: number;
}): string | null;
/**
 * Whether a stored credential blob still carries the access token we expect.
 *
 * Guards write-back against an external switch landing between the read that
 * produced the token being refreshed and the write of its replacement. An
 * unparseable blob returns false: a write into state we cannot identify is
 * exactly what this is meant to prevent.
 *
 * Deliberately stricter than the write it guards: this validates through
 * parseCredentials (field types, mcpOAuth-only rejection) while
 * updateCredentialBlob rewrites anything JSON.parse accepts. So a blob the
 * write path would happily update can still be refused here. That gap is
 * intentional — reaching this guard means an earlier parseCredentials
 * succeeded, so a blob that no longer parses changed shape after we read it
 * (a truncated non-atomic external write, or a hand edit), and declining to
 * write over it is correct. Anyone tightening parseCredentials should know it
 * narrows write-back too, not just reads.
 */
export declare function credentialBlobMatches(raw: string, expectedAccessToken: string): boolean;
export declare function writeBackCredentials(source: string, creds: ClaudeCredentials, configDir?: string, expectedPriorAccessToken?: string): boolean;
export declare function refreshAccount(source: string, configDir?: string): ClaudeCredentials | null;
//# sourceMappingURL=keychain.d.ts.map