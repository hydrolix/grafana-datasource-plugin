export default {
    components: {
        config: {
            editor: {
                host: {
                    label: "Server address",
                    description: "Hydrolix host address",
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
                skipTlsVerify: {
                    label: "Skip TLS Verify",
                    description: "Skip TLS Verify",
                },
                username: {
                    label: "Username",
                    description: "default",
                    tooltip: "Hydrolix username",
                },
                password: {
                    label: "Password",
                    description: "Hydrolix password",
                    placeholder: "Password",
                }
            }
        }
    }
}
