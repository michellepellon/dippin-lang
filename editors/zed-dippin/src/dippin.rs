use zed_extension_api::{self as zed, LanguageServerId, Result};

struct DippinExtension;

impl zed::Extension for DippinExtension {
    fn new() -> Self {
        DippinExtension
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        let path = worktree
            .which("dippin")
            .ok_or_else(|| "dippin not found on PATH; install with: go install github.com/2389-research/dippin-lang/cmd/dippin@latest".to_string())?;

        Ok(zed::Command {
            command: path,
            args: vec!["lsp".to_string()],
            env: Default::default(),
        })
    }
}

zed::register_extension!(DippinExtension);
