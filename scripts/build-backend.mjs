import { mkdirSync, readdirSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root = fileURLToPath(new URL("../", import.meta.url));
const backend = path.join(root, "backend");
mkdirSync(path.join(backend, "bin"), { recursive: true });
for (const command of readdirSync(path.join(backend, "cmd"), { withFileTypes: true }).filter(entry => entry.isDirectory())) {
  const result = spawnSync("go", ["build", "-o", path.join("bin", command.name + (process.platform === "win32" ? ".exe" : "")), "./cmd/" + command.name], { cwd: backend, stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
