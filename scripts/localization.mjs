import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "../frontend/node_modules/typescript/lib/typescript.js";

const root = fileURLToPath(new URL("../", import.meta.url));
const locales = ["de", "en", "es", "fr"];
const read = (relative) => fs.readFileSync(path.join(root, relative), "utf8");
const messages = Object.fromEntries(locales.map((locale) => [locale, JSON.parse(read(`localization/${locale}.json`))]));
const reference = messages.en;
const keys = Object.keys(reference).sort();
const placeholders = (text) => [...new Set([...text.matchAll(/{{(\w+)}}/g)].map((match) => match[1]))].sort();
const fail = (message) => { throw new Error(message); };
for (const locale of locales) {
  const tree = ts.parseJsonText(`${locale}.json`, read(`localization/${locale}.json`));
  const properties = tree.statements[0]?.expression?.properties ?? [];
  const names = properties.map((property) => property.name.text);
  if (new Set(names).size !== names.length) fail(`${locale}: duplicate message keys`);
}
for (const locale of locales) {
  const catalog = messages[locale];
  const missing = keys.filter((key) => !Object.hasOwn(catalog, key));
  const extra = Object.keys(catalog).filter((key) => !Object.hasOwn(reference, key));
  if (missing.length || extra.length) fail(`${locale}: missing [${missing}], extra [${extra}]`);
  for (const key of keys) {
    if (!/^[a-z][a-zA-Z0-9]*(\.[a-z][a-zA-Z0-9_]+)+$/.test(key)) fail(`Invalid key: ${key}`);
    const value = catalog[key];
    if (typeof value !== "string" || !value.trim()) fail(`${locale}.${key}: empty/non-string message`);
    if (/<\/?[a-z]/i.test(value)) fail(`${locale}.${key}: HTML is not allowed`);
    if (JSON.stringify(placeholders(value)) !== JSON.stringify(placeholders(reference[key]))) fail(`${locale}.${key}: placeholder mismatch`);
    if (value.replace(/{{\w+}}/g, "").includes("{{")) fail(`${locale}.${key}: malformed placeholder`);
  }
  for (const key of keys.filter((key) => key.endsWith("_one"))) {
    const base = key.slice(0, -4);
    for (const category of new Intl.PluralRules(locale).resolvedOptions().pluralCategories) {
      if (!Object.hasOwn(catalog, `${base}_${category}`)) fail(`${locale}: missing plural ${base}_${category}`);
    }
  }
}

const resourceName = (key) => key.replaceAll(".", "_");
for (const file of ["service.go", "progress.go", "google.go", "mobile.go"]) {
  const source = read(`backend/internal/accounts/${file}`);
  for (const match of source.matchAll(/jsonError\(w,\s*[^,]+,\s*"([a-z_]+)"|"code":\s*"([a-z_]+)"/g)) {
    const code = match[1] ?? match[2];
    if (!Object.hasOwn(reference, `api.${code}`)) fail(`Missing translation for server error code: ${code}`);
  }
}
if (new Set(keys.map(resourceName)).size !== keys.length) fail("Keys collide when converted to native resource names");
const xml = (value) => value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "\\'").replaceAll("\n", "\\n");
const base = "mobile/composeApp/src/commonMain";
const generated = {};
const pluralSuffix = /_(zero|one|two|few|many|other)$/;
const ordinary = keys.filter((key) => !pluralSuffix.test(key));
const known = new Set(keys.concat(keys.map((key) => key.replace(pluralSuffix, ""))));
const walk = (directory) => fs.readdirSync(path.join(root, directory), { withFileTypes: true }).flatMap((entry) => {
  const relative = `${directory}/${entry.name}`;
  return entry.isDirectory() ? walk(relative) : [relative];
});
const files = [...walk("frontend/src"), ...walk(`${base}/kotlin`), ...walk("mobile/androidApp/src/main/kotlin")]
  .filter((file) => /\.(ts|tsx|kt)$/.test(file) && !file.includes(".test.") && !file.endsWith("LocalizedStrings.kt"));
for (const file of files) {
  const source = read(file);
  for (const match of source.matchAll(/\b(?:t|message|localized)\(\s*"([a-z][a-zA-Z0-9_.]+)"/g)) {
    if (!known.has(match[1])) fail(`${file}: unknown message ${match[1]}`);
  }
  for (const match of source.matchAll(/\bMessage\.([a-zA-Z0-9_]+)/g)) {
    if (!ordinary.some((key) => resourceName(key) === match[1])) fail(`${file}: unknown native message ${match[1]}`);
  }
  if (file.endsWith(".tsx")) {
    const tree = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
    const check = (node) => {
      if (ts.isJsxText(node)) {
        const text = node.text.replace(/\s+/g, " ").trim();
        if (/\p{L}/u.test(text) && !["reel canon", "TMDb", "IMDb"].includes(text)) fail(`${file}: extract UI text into localization: ${text}`);
      }
      if (ts.isJsxAttribute(node) && ["aria-label", "alt", "title", "placeholder"].includes(node.name.text) && node.initializer && ts.isStringLiteral(node.initializer) && /\p{L}/u.test(node.initializer.text)) fail(`${file}: extract ${node.name.text} into localization`);
      ts.forEachChild(node, check);
    };
    check(tree);
  }
}
const pluralBases = [...new Set(keys.filter((key) => pluralSuffix.test(key)).map((key) => key.replace(pluralSuffix, "")))];
for (const locale of locales) {
  const strings = ordinary.map((key) => `    <string name="${resourceName(key)}">${xml(messages[locale][key])}</string>`);
  for (const key of pluralBases) {
    const items = ["zero", "one", "two", "few", "many", "other"].filter((category) => messages[locale][`${key}_${category}`]).map((category) => `        <item quantity="${category}">${xml(messages[locale][`${key}_${category}`])}</item>`);
    strings.push(`    <plurals name="${resourceName(key)}">\n${items.join("\n")}\n    </plurals>`);
  }
  generated[`${base}/composeResources/values${locale === "en" ? "" : `-${locale}`}/strings.xml`] = `<!-- Generated from localization/${locale}.json by scripts/localization.mjs. Do not edit. -->\n<resources>\n${strings.join("\n")}\n</resources>\n`;
}
generated[`${base}/kotlin/com/reelcanon/mobile/LocalizedStrings.kt`] = `// Generated by scripts/localization.mjs. Edit localization/*.json instead.\npackage com.reelcanon.mobile\n\nimport androidx.compose.runtime.Composable\nimport androidx.compose.runtime.staticCompositionLocalOf\nimport org.jetbrains.compose.resources.StringResource\nimport org.jetbrains.compose.resources.stringResource\nimport com.reelcanon.shared.generated.resources.*\n\nenum class Message(val resource: StringResource) {\n${ordinary.map((key) => `    ${resourceName(key)}(Res.string.${resourceName(key)}),`).join("\n")}\n}\n\nclass Strings(private val values: Map<Message, String>) {\n    operator fun invoke(message: Message, vararg arguments: Pair<String, Any?>): String {\n        val parameters = arguments.toMap()\n        return Regex("\\\\{\\\\{(\\\\w+)\\\\}\\\\}").replace(values.getValue(message)) { match ->\n            parameters[match.groupValues[1]]?.toString() ?: match.value\n        }\n    }\n}\n\nval LocalStrings = staticCompositionLocalOf<Strings> { error("Missing localized strings") }\n\n@Composable\nfun useStrings(): Strings {\n    val values = Message.entries.associateWith { stringResource(it.resource) }\n    return androidx.compose.runtime.remember(values) { Strings(values) }\n}\n`;

const write = process.argv.includes("--write");
for (const [relative, content] of Object.entries(generated)) {
  const target = path.join(root, relative);
  if (write) {
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, content);
  } else if (!fs.existsSync(target) || read(relative).replaceAll("\r\n", "\n") !== content) {
    fail(`Stale generated resource: ${relative}. Run node scripts/localization.mjs --write`);
  }
}
console.log(`Localization checked: ${locales.join(", ")}, ${keys.length} messages; native resources ${write ? "generated" : "current"}.`);
