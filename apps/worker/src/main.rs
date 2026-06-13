//! CLI entrypoint for git-dep-worker.
//!
//! Usage:
//!   git-dep-worker --root <dir> [--api <base_url> --token <jwt> --daemon --interval <secs> --token-url <url>]
//!
//! `<dir>` contains one subdirectory per repository, named by repo id (e.g.
//! `1/`, `2/`). The worker reads each repo's `go.mod`/`package.json`, computes
//! cross-repository [`DependencyLink`]s, prints `{"links":[...]}` to stdout,
//! and — when both `--api` and `--token` (or `--token-url`) are given — POSTs
//! them to `<base_url>/api/v1/dependency-links` with a Bearer token.

use std::path::{Path, PathBuf};
use std::process::ExitCode;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use git_dep_worker::{compute_links, scan_root, LinksEnvelope};

struct Args {
    root: PathBuf,
    api: Option<String>,
    token: Option<String>,
    daemon: bool,
    interval: u64,
    token_url: Option<String>,
}

fn parse_args() -> Result<Args, String> {
    let mut root: Option<PathBuf> = None;
    let mut api: Option<String> = None;
    let mut token: Option<String> = None;
    let mut daemon = false;
    let mut interval = 3600;
    let mut token_url: Option<String> = None;

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
            "--daemon" => daemon = true,
            "--interval" => {
                let val = it.next().ok_or("--interval requires a value")?;
                interval = val.parse::<u64>().map_err(|e| format!("invalid interval: {e}"))?;
            }
            "--token-url" => token_url = Some(it.next().ok_or("--token-url requires a value")?),
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
        daemon,
        interval,
        token_url,
    })
}

fn print_usage() {
    eprintln!(
        "git-dep-worker --root <dir> [--api <base_url> --token <jwt> --daemon --interval <secs> --token-url <url>]\n\
         \n\
         Scans one subdirectory per repository under <dir> (subdir name = repo id),\n\
         parses go.mod / package.json, and emits cross-repository dependency links\n\
         as {{\"links\":[...]}} on stdout. With --api and --token (or --token-url) it also POSTs them\n\
         to <base_url>/api/v1/dependency-links using the Bearer token."
    );
}

fn get_token(login_url: &str) -> Result<String, String> {
    let agent = ureq::AgentBuilder::new()
        .timeout(Duration::from_secs(10))
        .build();

    match agent.get(login_url).call() {
        Ok(resp) => {
            let body: serde_json::Value = resp.into_json().map_err(|e| format!("parse token json: {e}"))?;
            let token = body["access_token"]
                .as_str()
                .ok_or_else(|| "missing access_token in login response".to_string())?;
            Ok(token.to_string())
        }
        Err(e) => Err(format!("fetch token from {login_url} failed: {e}")),
    }
}

fn run_scan_and_post(
    root: &Path,
    api: Option<&str>,
    token: &mut Option<String>,
    token_url: Option<&str>,
) -> Result<(), String> {
    let repos = scan_root(root)?;
    let links = compute_links(&repos);
    let envelope = LinksEnvelope { links };

    let json = serde_json::to_string(&envelope).map_err(|e| format!("serialize links: {e}"))?;
    println!("{json}");

    if let Some(base) = api {
        if token.is_none() {
            if let Some(t_url) = token_url {
                *token = Some(get_token(t_url)?);
            }
        }

        let current_token = token.as_deref().ok_or_else(|| "--token or --token-url is required".to_string())?;

        match post_links(base, current_token, &envelope) {
            Ok(()) => {}
            Err(e) if (e.contains("401") || e.contains("403")) && token_url.is_some() => {
                eprintln!("POST failed with auth error, attempting token refresh...");
                let t_url = token_url.unwrap();
                let new_tok = get_token(t_url)?;
                *token = Some(new_tok.clone());
                post_links(base, &new_tok, &envelope)?;
            }
            Err(e) => return Err(e),
        }
    }

    Ok(())
}

fn run() -> Result<(), String> {
    let args = parse_args()?;
    let token_state = Arc::new(Mutex::new(args.token));
    let root = args.root;
    let api = args.api;
    let token_url = args.token_url;

    if args.daemon {
        let root_c = root.clone();
        let api_c = api.clone();
        let token_c = Arc::clone(&token_state);
        let token_url_c = token_url.clone();

        thread::spawn(move || {
            let listener = std::net::TcpListener::bind("0.0.0.0:8081");
            match listener {
                Ok(l) => {
                    eprintln!("Daemon trigger listener started on 0.0.0.0:8081");
                    for stream in l.incoming() {
                        if let Ok(mut stream) = stream {
                            eprintln!("Trigger received, starting scan...");
                            let mut tok_guard = token_c.lock().unwrap();
                            match run_scan_and_post(&root_c, api_c.as_deref(), &mut *tok_guard, token_url_c.as_deref()) {
                                Ok(()) => eprintln!("Triggered scan complete."),
                                Err(e) => eprintln!("Triggered scan failed: {e}"),
                            }
                            use std::io::Write;
                            let _ = stream.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK");
                        }
                    }
                }
                Err(e) => eprintln!("Failed to bind trigger listener: {e}"),
            }
        });

        loop {
            let mut tok_guard = token_state.lock().unwrap();
            eprintln!("Starting periodic scan...");
            if let Err(e) = run_scan_and_post(&root, api.as_deref(), &mut *tok_guard, token_url.as_deref()) {
                eprintln!("Periodic scan failed: {e}");
            }
            drop(tok_guard);

            thread::sleep(Duration::from_secs(args.interval));
        }
    } else {
        let mut tok_guard = token_state.lock().unwrap();
        run_scan_and_post(&root, api.as_deref(), &mut *tok_guard, token_url.as_deref())?;
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
