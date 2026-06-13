//! CLI entrypoint for git-dep-worker.
//!
//! Usage:
//!   git-dep-worker --root <dir> [--api <base_url> --token <jwt>]
//!
//! `<dir>` contains one subdirectory per repository, named by repo id (e.g.
//! `1/`, `2/`). The worker reads each repo's `go.mod`/`package.json`, computes
//! cross-repository [`DependencyLink`]s, prints `{"links":[...]}` to stdout,
//! and — when both `--api` and `--token` are given — POSTs them to
//! `<base_url>/api/v1/dependency-links` with a Bearer token.

use std::path::PathBuf;
use std::process::ExitCode;
use std::time::Duration;

use git_dep_worker::{compute_links, scan_root, LinksEnvelope};

struct Args {
    root: PathBuf,
    api: Option<String>,
    token: Option<String>,
}

fn parse_args() -> Result<Args, String> {
    let mut root: Option<PathBuf> = None;
    let mut api: Option<String> = None;
    let mut token: Option<String> = None;

    let mut it = std::env::args().skip(1);
    while let Some(arg) = it.next() {
        match arg.as_str() {
            "--root" => {
                root = Some(PathBuf::from(
                    it.next().ok_or("--root requires a value")?,
                ));
            }
            "--api" => api = Some(it.next().ok_or("--api requires a value")?),
            "--token" => token = Some(it.next().ok_or("--token requires a value")?),
            "-h" | "--help" => {
                print_usage();
                std::process::exit(0);
            }
            other => return Err(format!("unknown argument: {other}")),
        }
    }

    Ok(Args {
        root: root.ok_or("--root is required")?,
        api,
        token,
    })
}

fn print_usage() {
    eprintln!(
        "git-dep-worker --root <dir> [--api <base_url> --token <jwt>]\n\
         \n\
         Scans one subdirectory per repository under <dir> (subdir name = repo id),\n\
         parses go.mod / package.json, and emits cross-repository dependency links\n\
         as {{\"links\":[...]}} on stdout. With --api and --token it also POSTs them\n\
         to <base_url>/api/v1/dependency-links using the Bearer token."
    );
}

fn run() -> Result<(), String> {
    let args = parse_args()?;

    let repos = scan_root(&args.root)?;
    let links = compute_links(&repos);
    let envelope = LinksEnvelope { links };

    let json = serde_json::to_string(&envelope).map_err(|e| format!("serialize links: {e}"))?;
    println!("{json}");

    // Only POST when BOTH --api and --token are present.
    match (args.api, args.token) {
        (Some(base), Some(token)) => post_links(&base, &token, &envelope)?,
        (Some(_), None) | (None, Some(_)) => {
            return Err("--api and --token must be provided together".to_string());
        }
        (None, None) => {}
    }

    Ok(())
}

/// POSTs the links envelope to `<base>/api/v1/dependency-links` with a Bearer
/// token and logs the response status (and `stored` count when present). A
/// non-2xx response is a hard error.
fn post_links(base: &str, token: &str, envelope: &LinksEnvelope) -> Result<(), String> {
    let url = format!("{}/api/v1/dependency-links", base.trim_end_matches('/'));
    let agent = ureq::AgentBuilder::new()
        .timeout(Duration::from_secs(30))
        .build();

    match agent
        .post(&url)
        .set("Authorization", &format!("Bearer {token}"))
        .send_json(envelope)
    {
        Ok(resp) => {
            let status = resp.status();
            let body = resp.into_string().unwrap_or_default();
            eprintln!("POST {url} -> {status} {body}");
            Ok(())
        }
        Err(ureq::Error::Status(code, resp)) => {
            let body = resp.into_string().unwrap_or_default();
            Err(format!("POST {url} failed: {code} {body}"))
        }
        Err(e) => Err(format!("POST {url} transport error: {e}")),
    }
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            eprintln!("git-dep-worker: error: {e}");
            ExitCode::FAILURE
        }
    }
}
