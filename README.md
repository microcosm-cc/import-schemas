Import the format produced by `export-schemas` into microcosm.

To compile, run `go build`. Modify `config.toml` to reflect the exports directory, site attributes and db credentials.

Then run `./import-schemas` to run the import tool.

`Store` functions are idempotent. For example, if a user has already been imported and `StoreUser` is called again with the same parameters, this is not considered an error and should return the user's ID. This means the import tool can be run multiple times on the same (or slightly differing) dataset.
