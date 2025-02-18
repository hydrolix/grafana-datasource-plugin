export default {
    components: {
        config: {
            editor: {
                host: {
                    label: "Server address",
                    description: "Hydrolix server address",
                    placeholder: "Server address",
                    error: "Server address required"
                },
                port: {
                    label: "Server port",
                    description: "Hydrolix server port",
                    placeholder: "Server port",
                    error: "Server port required",
                    insecureNativePort: "9000",
                    insecureHttpPort: "8123",
                    secureNativePort: "9440",
                    secureHttpPort: "8443"
                },
                protocol: {
                    label: "Protocol",
                    description: "Native or HTTP for server protocol",
                },
                secure: {
                    label: "Secure Connection",
                    description: "Toggle on if the connection is secure",
                },
                path: {
                    label: "HTTP URL Path",
                    description: "Additional URL path for HTTP requests",
                    placeholder: "additional-path"
                },
                skipTlsVerify: {
                    label: "Skip TLS Verify",
                    description: "Skip TLS Verify",
                },
                username: {
                    label: "Username",
                    description: "Hydrolix username",
                    placeholder: "default",
                },
                password: {
                    label: "Password",
                    description: "Hydrolix password",
                    placeholder: "Password",
                },
                dialTimeout: {
                    label: "Dial Timeout (seconds)",
                    description: "Timeout in seconds for connection",
                    placeholder: "10"
                },
                queryTimeout: {
                    label: "Query Timeout (seconds)",
                    description: "Timeout in seconds for read queries",
                    placeholder: "60"
                },
                defaultDatabase: {
                    label: "Default database",
                    description: "Used when no specific database is selected in queries",
                    placeholder: "sample"
                }
            }
        }
    }
}
