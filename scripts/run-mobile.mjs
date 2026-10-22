import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const tasks = {
  check: [':androidApp:lintDebug', ':composeApp:testAndroidHostTest', ':androidApp:assembleDebug'],
  test: [':composeApp:testAndroidHostTest'],
  debug: [':androidApp:assembleDebug'],
  device: [':androidApp:connectedDebugAndroidTest'],
  bundle: [':androidApp:bundleRelease'],
};
const [command, ...extra] = process.argv.slice(2);
if (!tasks[command]) throw new Error(`Unknown mobile target: ${command}`);
const cwd = fileURLToPath(new URL('../mobile/', import.meta.url));
const windows = process.platform === 'win32';
const result = spawnSync(windows ? 'gradlew.bat' : './gradlew', [...tasks[command], ...extra], {
  cwd, stdio: 'inherit', shell: windows,
});
if (result.error) throw result.error;
process.exit(result.status ?? 1);
