package com.example

class PlatformGrpcTargetsFlags {
    class EnvironmentSpec {
        /**
         * Which Environment to use for connecting to services.
         */
        @CommandLine.Option(
            names = ["--environment", "--env"],
            arity = "0..1",
            description = [
                "Which environment to use for connecting to services (LOCAL, DEV, STAGING, PROD, DEV_OLD, PROD_OLD). Default is LOCAL.",
            ],
        )
        var environment: Environment = Environment.LOCAL
    }
}
