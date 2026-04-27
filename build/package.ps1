param(
    [string]$PluginId = "ai.flexing.mattermost-plugin-remind",
    [string]$PluginVersion = "1.0.0"
)

$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$dist = Join-Path $root "dist"
$bundleRoot = Join-Path $dist $PluginId
$bundlePath = Join-Path $dist "$PluginId-$PluginVersion.tar.gz"
$executables = @(
    "server/dist/plugin-linux-amd64",
    "server/dist/plugin-linux-arm64",
    "server/dist/plugin-darwin-amd64",
    "server/dist/plugin-darwin-arm64",
    "server/dist/plugin-windows-amd64.exe"
)

$script = @"
import shutil
import tarfile
from pathlib import Path

root = Path(r"$root")
dist = Path(r"$dist")
bundle_root = Path(r"$bundleRoot")
bundle_path = Path(r"$bundlePath")
plugin_id = "$PluginId"
executables = set($($executables | ConvertTo-Json -Compress))

if bundle_root.exists():
    shutil.rmtree(bundle_root)
bundle_root.mkdir(parents=True, exist_ok=True)

shutil.copy2(root / "plugin.json", bundle_root / "plugin.json")
shutil.copytree(root / "assets", bundle_root / "assets")
(bundle_root / "server").mkdir()
shutil.copytree(root / "server" / "dist", bundle_root / "server" / "dist")

with tarfile.open(bundle_path, "w:gz", format=tarfile.GNU_FORMAT) as tf:
    root_info = tarfile.TarInfo(plugin_id + "/")
    root_info.type = tarfile.DIRTYPE
    root_info.mode = 0o755
    root_info.uid = root_info.gid = 0
    root_info.uname = root_info.gname = ""
    tf.addfile(root_info)

    for path in sorted(bundle_root.rglob("*")):
        rel = path.relative_to(bundle_root).as_posix()
        info = tf.gettarinfo(str(path), arcname=f"{plugin_id}/{rel}")
        info.uid = info.gid = 0
        info.uname = info.gname = ""
        info.mode = 0o755 if path.is_dir() or rel in executables else 0o644
        if path.is_dir():
            tf.addfile(info)
        else:
            with path.open("rb") as f:
                tf.addfile(info, f)

print(bundle_path)
"@

$script | python -
