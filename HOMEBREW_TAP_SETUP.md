# Homebrew Tap Setup

## 1. Create the tap repository

Create a new public repository at `e-invoice-be/homebrew-tap` on GitHub.

Add a minimal `README.md`:

```markdown
# e-invoice-be Homebrew Tap

## Usage

brew tap e-invoice-be/tap
brew install peppol
```

## 2. Create a Personal Access Token

1. Go to https://github.com/settings/tokens
2. Generate a new **fine-grained** token with:
   - Repository access: `e-invoice-be/homebrew-tap` only
   - Permissions: Contents (read and write)
3. Copy the token

## 3. Add the secret to peppol-cli

1. Go to https://github.com/e-invoice-be/peppol-cli/settings/secrets/actions
2. Add a new repository secret:
   - Name: `HOMEBREW_TAP_TOKEN`
   - Value: the token from step 2

## 4. Test

After pushing a `v*` tag to peppol-cli, goreleaser will automatically update the formula in the tap repository.

```bash
brew tap e-invoice-be/tap
brew install peppol
peppol version
```
