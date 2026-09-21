use serde::Serialize;
use sha2::{Digest, Sha256};
use std::env;
use std::fs::File;
use std::io::Read;
use std::path::{Component, Path};
use std::process::ExitCode;
use zip::read::ZipArchive;

const EXPECTED_DATABASE_ENTRY: &str = "pb_data/data.db";
const MAX_ARCHIVE_ENTRIES: usize = 10_000;

#[derive(Debug, Serialize)]
struct BackupManifest {
    archive: String,
    archive_sha256: String,
    archive_bytes: u64,
    zip_valid: bool,
    entry_count: usize,
    expected_database_entry: &'static str,
    database_entry_present: bool,
    database_entry_bytes: Option<u64>,
}

fn main() -> ExitCode {
    let mut args = env::args_os();
    let program = args.next().unwrap_or_default();
    let Some(path) = args.next() else {
        eprintln!(
            "Usage: {} <pocketbase-backup.zip>",
            Path::new(&program).display()
        );
        return ExitCode::from(64);
    };
    if args.next().is_some() {
        eprintln!("Expected exactly one backup archive path.");
        return ExitCode::from(64);
    }

    match inspect_backup(Path::new(&path)) {
        Ok(manifest) => match serde_json::to_string_pretty(&manifest) {
            Ok(json) => {
                println!("{json}");
                ExitCode::SUCCESS
            }
            Err(error) => {
                eprintln!("Could not serialize manifest: {error}");
                ExitCode::from(70)
            }
        },
        Err(error) => {
            eprintln!("Backup integrity check failed: {error}");
            ExitCode::from(65)
        }
    }
}

fn inspect_backup(path: &Path) -> Result<BackupManifest, String> {
    let metadata = std::fs::metadata(path)
        .map_err(|error| format!("cannot read {}: {error}", path.display()))?;
    if !metadata.is_file() {
        return Err(format!("{} is not a regular file", path.display()));
    }

    let archive_sha256 = sha256_file(path)?;
    let archive_file = File::open(path)
        .map_err(|error| format!("cannot open {} as ZIP: {error}", path.display()))?;
    let mut archive = ZipArchive::new(archive_file)
        .map_err(|error| format!("{} is not a valid ZIP archive: {error}", path.display()))?;

    if archive.len() > MAX_ARCHIVE_ENTRIES {
        return Err(format!(
            "archive has {} entries; maximum allowed is {MAX_ARCHIVE_ENTRIES}",
            archive.len()
        ));
    }

    let mut database_entry_bytes = None;
    for index in 0..archive.len() {
        let entry = archive
            .by_index(index)
            .map_err(|error| format!("cannot inspect ZIP entry {index}: {error}"))?;
        let name = entry.name();
        if !is_safe_relative_path(name) {
            return Err(format!("unsafe archive entry path: {name}"));
        }
        if is_symlink(&entry) {
            return Err(format!("symbolic-link archive entries are not accepted: {name}"));
        }
        if name == EXPECTED_DATABASE_ENTRY {
            database_entry_bytes = Some(entry.size());
        }
    }

    if database_entry_bytes.is_none() {
        return Err(format!(
            "required PocketBase database entry {EXPECTED_DATABASE_ENTRY} is missing"
        ));
    }

    Ok(BackupManifest {
        archive: path.display().to_string(),
        archive_sha256,
        archive_bytes: metadata.len(),
        zip_valid: true,
        entry_count: archive.len(),
        expected_database_entry: EXPECTED_DATABASE_ENTRY,
        database_entry_present: true,
        database_entry_bytes,
    })
}

fn sha256_file(path: &Path) -> Result<String, String> {
    let mut file = File::open(path)
        .map_err(|error| format!("cannot open {} for hashing: {error}", path.display()))?;
    let mut hasher = Sha256::new();
    let mut buffer = [0_u8; 64 * 1024];
    loop {
        let read = file
            .read(&mut buffer)
            .map_err(|error| format!("cannot hash {}: {error}", path.display()))?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    Ok(format!("{:x}", hasher.finalize()))
}

fn is_safe_relative_path(name: &str) -> bool {
    if name.is_empty() || name.contains('\\') {
        return false;
    }
    let path = Path::new(name);
    if path.is_absolute() {
        return false;
    }
    path.components().all(|component| {
        matches!(component, Component::Normal(_) | Component::CurDir)
            && !matches!(component, Component::ParentDir | Component::RootDir | Component::Prefix(_))
    })
}

fn is_symlink(entry: &zip::read::ZipFile<'_>) -> bool {
    entry
        .unix_mode()
        .map(|mode| mode & 0o170000 == 0o120000)
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::tempdir;
    use zip::write::FileOptions;
    use zip::ZipWriter;

    #[test]
    fn accepts_archive_with_pocketbase_database_and_manifest() {
        let directory = tempdir().expect("temporary directory");
        let path = directory.path().join("backup.zip");
        let file = File::create(&path).expect("create archive");
        let mut writer = ZipWriter::new(file);
        writer
            .start_file(EXPECTED_DATABASE_ENTRY, FileOptions::default())
            .expect("start database entry");
        writer.write_all(b"SQLite format 3\0").expect("write database");
        writer.finish().expect("finish archive");

        let manifest = inspect_backup(&path).expect("valid backup");
        assert!(manifest.zip_valid);
        assert!(manifest.database_entry_present);
        assert_eq!(manifest.database_entry_bytes, Some(16));
        assert_eq!(manifest.archive_sha256.len(), 64);
    }

    #[test]
    fn rejects_archive_without_database_entry() {
        let directory = tempdir().expect("temporary directory");
        let path = directory.path().join("missing-db.zip");
        let file = File::create(&path).expect("create archive");
        let mut writer = ZipWriter::new(file);
        writer
            .start_file("notes.txt", FileOptions::default())
            .expect("start note entry");
        writer.write_all(b"not a backup").expect("write note");
        writer.finish().expect("finish archive");

        let error = inspect_backup(&path).expect_err("missing database must fail");
        assert!(error.contains(EXPECTED_DATABASE_ENTRY));
    }

    #[test]
    fn rejects_path_traversal_entries() {
        let directory = tempdir().expect("temporary directory");
        let path = directory.path().join("unsafe.zip");
        let file = File::create(&path).expect("create archive");
        let mut writer = ZipWriter::new(file);
        writer
            .start_file("../pb_data/data.db", FileOptions::default())
            .expect("start unsafe entry");
        writer.write_all(b"SQLite format 3\0").expect("write database");
        writer.finish().expect("finish archive");

        let error = inspect_backup(&path).expect_err("unsafe path must fail");
        assert!(error.contains("unsafe archive entry path"));
    }
}
