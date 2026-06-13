//! git-dep-worker — Semantic AST / dependency parser worker.
//!
//! Parses per-repository Go (`go.mod`) and npm (`package.json`) manifests and
//! auto-generates cross-repository dependency links. The parsing and matching
//! logic lives here (no network, minimal filesystem) so it is unit-testable;
//! the CLI wiring and HTTP POST live in `main.rs`.
//!
//! ## Shared contract
//! A [`DependencyLink`] is the exact JSON the backend ingests and the frontend
//! renders: `from_repo` depends on `to_repo` through module/package `via`,
//! classified by `kind` ("go" | "npm"). Repo ids are the directory names under
//! the scanned root (one subdirectory per repository, named by its repo id).

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

/// A cross-repository dependency: `from_repo` depends on `to_repo` through
/// module/package `via`. `kind` is "go" or "npm". Field names are snake_case
/// to match the backend/frontend contract.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub struct DependencyLink {
    pub from_repo: String,
    pub to_repo: String,
    pub via: String,
    pub kind: String,
}

/// The JSON envelope for the links: `{"links":[...]}`. Matches the backend
/// POST body and GET response shape.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LinksEnvelope {
    pub links: Vec<DependencyLink>,
}

/// What a single repository provides and depends on, extracted from its
/// manifests. `provided_*` are modules/packages this repo PUBLISHES;
/// `*_deps` are what it consumes.
#[derive(Debug, Default, Clone, PartialEq, Eq)]
pub struct RepoManifest {
    /// Go module path declared by the `module` line of go.mod, if any.
    pub go_module: Option<String>,
    /// Go module paths from go.mod `require` entries (block or single-line).
    pub go_deps: Vec<String>,
    /// npm package name declared by package.json `.name`, if any.
    pub npm_name: Option<String>,
    /// npm dependency names from `.dependencies` and `.devDependencies`.
    pub npm_deps: Vec<String>,
}

/// Parses a `go.mod` file's contents, extracting the declared module path and
/// every `require`d module path. Handles both the block form
/// (`require (\n  path v1\n)`) and the single-line form (`require path v1`).
/// Comments (`//`) and blank lines are ignored.
pub fn parse_go_mod(contents: &str) -> (Option<String>, Vec<String>) {
    let mut module: Option<String> = None;
    let mut deps: Vec<String> = Vec::new();
    let mut in_require_block = false;

    for raw in contents.lines() {
        // Strip line comments, then trim.
        let line = match raw.find("//") {
            Some(idx) => &raw[..idx],
            None => raw,
        };
        let line = line.trim();
        if line.is_empty() {
            continue;
        }

        if in_require_block {
            if line == ")" {
                in_require_block = false;
                continue;
            }
            if let Some(path) = first_token(line) {
                deps.push(path.to_string());
            }
            continue;
        }

        if let Some(rest) = line.strip_prefix("module") {
            let rest = rest.trim();
            if !rest.is_empty() {
                module = Some(rest.to_string());
            }
            continue;
        }

        if let Some(rest) = line.strip_prefix("require") {
            let rest = rest.trim();
            if rest == "(" {
                in_require_block = true;
            } else if !rest.is_empty() {
                // Single-line require: `require <path> <version>`.
                if let Some(path) = first_token(rest) {
                    deps.push(path.to_string());
                }
            }
            continue;
        }
    }

    (module, deps)
}

/// Returns the first whitespace-delimited token of `s`, if any.
fn first_token(s: &str) -> Option<&str> {
    s.split_whitespace().next()
}

/// Parses a `package.json`'s contents, extracting `.name` and the union of
/// `.dependencies` and `.devDependencies` keys. Malformed JSON yields an Err.
pub fn parse_package_json(contents: &str) -> Result<(Option<String>, Vec<String>), serde_json::Error> {
    #[derive(Deserialize)]
    struct PkgJson {
        name: Option<String>,
        #[serde(default)]
        dependencies: BTreeMap<String, serde_json::Value>,
        #[serde(default, rename = "devDependencies")]
        dev_dependencies: BTreeMap<String, serde_json::Value>,
    }
    let pkg: PkgJson = serde_json::from_str(contents)?;
    let mut deps: Vec<String> = Vec::new();
    for k in pkg.dependencies.keys() {
        deps.push(k.clone());
    }
    for k in pkg.dev_dependencies.keys() {
        deps.push(k.clone());
    }
    Ok((pkg.name, deps))
}

/// Reads the `go.mod` and `package.json` (when present) inside `repo_dir` and
/// returns the combined [`RepoManifest`]. Missing manifests are NOT errors —
/// they simply leave the corresponding fields empty. Returns Err only on a
/// real I/O failure reading a file that exists or malformed package.json.
pub fn read_repo_manifest(repo_dir: &Path) -> Result<RepoManifest, String> {
    let mut m = RepoManifest::default();

    let go_mod_path = repo_dir.join("go.mod");
    if go_mod_path.is_file() {
        let contents = fs::read_to_string(&go_mod_path)
            .map_err(|e| format!("read {}: {e}", go_mod_path.display()))?;
        let (module, deps) = parse_go_mod(&contents);
        m.go_module = module;
        m.go_deps = deps;
    }

    let pkg_path = repo_dir.join("package.json");
    if pkg_path.is_file() {
        let contents = fs::read_to_string(&pkg_path)
            .map_err(|e| format!("read {}: {e}", pkg_path.display()))?;
        let (name, deps) = parse_package_json(&contents)
            .map_err(|e| format!("parse {}: {e}", pkg_path.display()))?;
        m.npm_name = name;
        m.npm_deps = deps;
    }

    Ok(m)
}

/// Scans `root` for one subdirectory per repository (subdirectory NAME = repo
/// id) and reads each one's manifest. Returns a stable (sorted-by-id) list of
/// `(repo_id, RepoManifest)`. Non-directory entries are ignored.
pub fn scan_root(root: &Path) -> Result<Vec<(String, RepoManifest)>, String> {
    let mut out: Vec<(String, RepoManifest)> = Vec::new();
    let entries =
        fs::read_dir(root).map_err(|e| format!("read root {}: {e}", root.display()))?;
    for entry in entries {
        let entry = entry.map_err(|e| format!("read dir entry: {e}"))?;
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }
        let id = match path.file_name().and_then(|s| s.to_str()) {
            Some(name) => name.to_string(),
            None => continue,
        };
        let manifest = read_repo_manifest(&path)?;
        out.push((id, manifest));
    }
    out.sort_by(|a, b| a.0.cmp(&b.0));
    Ok(out)
}

/// Computes cross-repository dependency links from per-repo manifests.
///
/// ## Algorithm
/// 1. Build provider maps `provided_module -> repo_id` separately for Go and
///    npm across all repos (a repo's go.mod `module` and package.json `.name`).
/// 2. For each repo's dependencies:
///    - **Go**: a dependency `d` matches provider module `p` when `d == p` OR
///      `d` is a sub-package of `p` (`d` starts with `p` + "/"). This prefix
///      rule is what lets `github.com/acme/lib/sub` resolve to the repo that
///      provides `github.com/acme/lib`. The most specific (longest) matching
///      provider wins.
///    - **npm**: exact match of the dependency key to a provider `.name`.
/// 3. Emit a [`DependencyLink`] `{from_repo, to_repo, via, kind}` where `via`
///    is the PROVIDER's module/package path. Self-links (from == to) are
///    dropped and duplicates are de-duplicated.
///
/// Results are returned sorted for determinism.
pub fn compute_links(repos: &[(String, RepoManifest)]) -> Vec<DependencyLink> {
    // Provider maps: module/package -> repo_id.
    let mut go_providers: BTreeMap<String, String> = BTreeMap::new();
    let mut npm_providers: BTreeMap<String, String> = BTreeMap::new();
    for (id, m) in repos {
        if let Some(module) = &m.go_module {
            go_providers.insert(module.clone(), id.clone());
        }
        if let Some(name) = &m.npm_name {
            npm_providers.insert(name.clone(), id.clone());
        }
    }

    let mut links: BTreeSet<DependencyLink> = BTreeSet::new();

    for (from_id, m) in repos {
        // Go deps: exact or longest-prefix module match.
        for dep in &m.go_deps {
            if let Some((via, to_id)) = match_go_provider(dep, &go_providers) {
                if &to_id != from_id {
                    links.insert(DependencyLink {
                        from_repo: from_id.clone(),
                        to_repo: to_id,
                        via,
                        kind: "go".to_string(),
                    });
                }
            }
        }
        // npm deps: exact name match.
        for dep in &m.npm_deps {
            if let Some(to_id) = npm_providers.get(dep) {
                if to_id != from_id {
                    links.insert(DependencyLink {
                        from_repo: from_id.clone(),
                        to_repo: to_id.clone(),
                        via: dep.clone(),
                        kind: "npm".to_string(),
                    });
                }
            }
        }
    }

    links.into_iter().collect()
}

/// Finds the provider module for a Go dependency: exact match wins, otherwise
/// the longest provider that `dep` is a sub-package of (`dep` == provider or
/// `dep` starts with provider + "/"). Returns `(provider_module, repo_id)`.
fn match_go_provider(
    dep: &str,
    providers: &BTreeMap<String, String>,
) -> Option<(String, String)> {
    let mut best: Option<(&String, &String)> = None;
    for (module, repo_id) in providers {
        let matches = dep == module || dep.starts_with(&format!("{module}/"));
        if matches {
            match best {
                Some((bm, _)) if bm.len() >= module.len() => {}
                _ => best = Some((module, repo_id)),
            }
        }
    }
    best.map(|(m, r)| (m.clone(), r.clone()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_mod_module_and_block_requires() {
        let src = "module github.com/acme/app\n\
                   \n\
                   go 1.22\n\
                   \n\
                   require (\n\
                   \tgithub.com/acme/lib v1.2.3\n\
                   \tgithub.com/other/thing v0.1.0 // indirect\n\
                   )\n";
        let (module, deps) = parse_go_mod(src);
        assert_eq!(module.as_deref(), Some("github.com/acme/app"));
        assert_eq!(
            deps,
            vec![
                "github.com/acme/lib".to_string(),
                "github.com/other/thing".to_string()
            ]
        );
    }

    #[test]
    fn go_mod_single_line_require() {
        let src = "module github.com/acme/app\n\
                   require github.com/acme/lib v1.0.0\n";
        let (module, deps) = parse_go_mod(src);
        assert_eq!(module.as_deref(), Some("github.com/acme/app"));
        assert_eq!(deps, vec!["github.com/acme/lib".to_string()]);
    }

    #[test]
    fn go_mod_no_module_or_requires() {
        let (module, deps) = parse_go_mod("go 1.22\n");
        assert_eq!(module, None);
        assert!(deps.is_empty());
    }

    #[test]
    fn package_json_name_and_deps() {
        let src = r#"{
            "name": "@acme/app",
            "version": "1.0.0",
            "dependencies": { "@acme/lib": "^1.0.0", "lodash": "^4.0.0" },
            "devDependencies": { "@acme/test-utils": "^0.1.0" }
        }"#;
        let (name, mut deps) = parse_package_json(src).expect("valid json");
        deps.sort();
        assert_eq!(name.as_deref(), Some("@acme/app"));
        assert_eq!(
            deps,
            vec![
                "@acme/lib".to_string(),
                "@acme/test-utils".to_string(),
                "lodash".to_string()
            ]
        );
    }

    #[test]
    fn package_json_missing_fields_ok() {
        let (name, deps) = parse_package_json(r#"{"version":"1.0.0"}"#).expect("valid json");
        assert_eq!(name, None);
        assert!(deps.is_empty());
    }

    #[test]
    fn package_json_malformed_errors() {
        assert!(parse_package_json("{ not json").is_err());
    }

    fn repo(id: &str, m: RepoManifest) -> (String, RepoManifest) {
        (id.to_string(), m)
    }

    #[test]
    fn go_exact_match_emits_link() {
        let repos = vec![
            repo(
                "1",
                RepoManifest {
                    go_module: Some("github.com/acme/lib".into()),
                    ..Default::default()
                },
            ),
            repo(
                "2",
                RepoManifest {
                    go_deps: vec!["github.com/acme/lib".into()],
                    ..Default::default()
                },
            ),
        ];
        let links = compute_links(&repos);
        assert_eq!(
            links,
            vec![DependencyLink {
                from_repo: "2".into(),
                to_repo: "1".into(),
                via: "github.com/acme/lib".into(),
                kind: "go".into(),
            }]
        );
    }

    #[test]
    fn go_prefix_match_resolves_subpackage() {
        let repos = vec![
            repo(
                "1",
                RepoManifest {
                    go_module: Some("github.com/acme/lib".into()),
                    ..Default::default()
                },
            ),
            repo(
                "2",
                RepoManifest {
                    go_deps: vec!["github.com/acme/lib/sub/pkg".into()],
                    ..Default::default()
                },
            ),
        ];
        let links = compute_links(&repos);
        assert_eq!(links.len(), 1);
        assert_eq!(links[0].via, "github.com/acme/lib");
        assert_eq!(links[0].from_repo, "2");
        assert_eq!(links[0].to_repo, "1");
    }

    #[test]
    fn go_prefix_does_not_falsely_match_sibling() {
        // "github.com/acme/library" must NOT match provider "github.com/acme/lib".
        let repos = vec![
            repo(
                "1",
                RepoManifest {
                    go_module: Some("github.com/acme/lib".into()),
                    ..Default::default()
                },
            ),
            repo(
                "2",
                RepoManifest {
                    go_deps: vec!["github.com/acme/library".into()],
                    ..Default::default()
                },
            ),
        ];
        assert!(compute_links(&repos).is_empty());
    }

    #[test]
    fn npm_exact_match_only() {
        let repos = vec![
            repo(
                "1",
                RepoManifest {
                    npm_name: Some("@acme/lib".into()),
                    ..Default::default()
                },
            ),
            repo(
                "2",
                RepoManifest {
                    npm_deps: vec!["@acme/lib".into(), "@acme/lib-extra".into()],
                    ..Default::default()
                },
            ),
        ];
        let links = compute_links(&repos);
        assert_eq!(links.len(), 1);
        assert_eq!(links[0].kind, "npm");
        assert_eq!(links[0].via, "@acme/lib");
    }

    #[test]
    fn no_self_links() {
        let repos = vec![repo(
            "1",
            RepoManifest {
                go_module: Some("github.com/acme/lib".into()),
                go_deps: vec!["github.com/acme/lib".into()],
                ..Default::default()
            },
        )];
        assert!(compute_links(&repos).is_empty());
    }

    #[test]
    fn no_duplicate_links() {
        // Two go deps that both resolve to the same provider produce one link.
        let repos = vec![
            repo(
                "1",
                RepoManifest {
                    go_module: Some("github.com/acme/lib".into()),
                    ..Default::default()
                },
            ),
            repo(
                "2",
                RepoManifest {
                    go_deps: vec![
                        "github.com/acme/lib".into(),
                        "github.com/acme/lib/sub".into(),
                    ],
                    ..Default::default()
                },
            ),
        ];
        // Both resolve to via=github.com/acme/lib -> one unique link.
        assert_eq!(compute_links(&repos).len(), 1);
    }

    #[test]
    fn longest_prefix_provider_wins() {
        // Provider repo "1" provides the parent, repo "3" provides the sub-module;
        // a dep on the sub path should attribute to repo "3" (most specific).
        let repos = vec![
            repo(
                "1",
                RepoManifest {
                    go_module: Some("github.com/acme/lib".into()),
                    ..Default::default()
                },
            ),
            repo(
                "3",
                RepoManifest {
                    go_module: Some("github.com/acme/lib/v2".into()),
                    ..Default::default()
                },
            ),
            repo(
                "2",
                RepoManifest {
                    go_deps: vec!["github.com/acme/lib/v2/inner".into()],
                    ..Default::default()
                },
            ),
        ];
        let links = compute_links(&repos);
        assert_eq!(links.len(), 1);
        assert_eq!(links[0].to_repo, "3");
        assert_eq!(links[0].via, "github.com/acme/lib/v2");
    }

    #[test]
    fn scan_root_reads_fixtures() {
        let dir = tempfile::tempdir().unwrap();
        let r1 = dir.path().join("1");
        let r2 = dir.path().join("2");
        std::fs::create_dir(&r1).unwrap();
        std::fs::create_dir(&r2).unwrap();
        std::fs::write(r1.join("go.mod"), "module github.com/acme/lib\n").unwrap();
        std::fs::write(r2.join("go.mod"), "module github.com/acme/app\nrequire github.com/acme/lib v1.0.0\n").unwrap();

        let repos = scan_root(dir.path()).unwrap();
        let links = compute_links(&repos);
        assert_eq!(links.len(), 1);
        assert_eq!(links[0].from_repo, "2");
        assert_eq!(links[0].to_repo, "1");
        assert_eq!(links[0].via, "github.com/acme/lib");
        assert_eq!(links[0].kind, "go");
    }

    #[test]
    fn scan_root_skips_repos_without_manifests() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::create_dir(dir.path().join("1")).unwrap();
        // No manifests at all -> empty manifest, no error, no links.
        let repos = scan_root(dir.path()).unwrap();
        assert_eq!(repos.len(), 1);
        assert!(compute_links(&repos).is_empty());
    }
}
