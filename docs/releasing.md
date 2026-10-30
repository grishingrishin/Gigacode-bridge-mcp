# Release procedure

The `.github/workflows/release.yml` workflow runs when a `vX.Y.Z` tag is pushed.
It verifies the code on macOS and Linux, builds four archives and SHA256SUMS, and creates a **draft** GitHub Release. Standard checks do not require GigaCode, provider authentication, or secrets. The GitHub Actions token is used only to create the release.

Before releasing, update the Implementation version in server.go, review the changes, and create a commit following the project's conventions. Creating and pushing a tag is a separate maintainer decision. After the workflow succeeds, review the draft assets and select Publish release. Draft assets are not available to ordinary visitors of the Releases page. Rerunning the workflow does not automatically overwrite an existing release.

Local preparation:

```sh
go test -race -shuffle=on -count=1 ./...
go vet ./...
bash scripts/package-release.sh v0.2.0
```

Output: `dist/v0.2.0/`. The script refuses to overwrite an existing directory. Each archive contains one executable, the README, documentation, illustrations, and the optional skill. Source code, local configuration, tokens, and Git history are excluded. Go is required only for building.

SHA256SUMS verifies download integrity; it is not a digital signature. macOS binaries are not yet signed or notarized. Check Gatekeeper behavior on the target machine; do not recommend disabling it globally. Cross-compilation does not replace execution on each target architecture.
