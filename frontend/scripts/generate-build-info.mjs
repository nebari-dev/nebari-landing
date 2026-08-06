import { execFile } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

async function readGitValue(args) {
  try {
    const { stdout } = await execFileAsync("git", args, { encoding: "utf8" });
    return stdout.trim();
  } catch {
    return "";
  }
}

async function readChartVersion() {
  try {
    const chart = await readFile("../charts/nebari-landing/Chart.yaml", "utf8");
    return chart.match(/^version:\s*["']?([^\s"']+)/m)?.[1] ?? "";
  } catch {
    return "";
  }
}

const commit = process.env.GIT_COMMIT?.trim() || (await readGitValue(["rev-parse", "HEAD"]));
const buildInfo = {
  version: process.env.APP_VERSION?.trim() || (await readChartVersion()) || "dev",
  commit: commit ? commit.slice(0, 7) : "unknown",
  lastUpdated:
    process.env.GIT_COMMIT_DATE?.trim() ||
    (await readGitValue(["show", "-s", "--format=%cI", "HEAD"])) ||
    null,
};

await writeFile("public/build-info.json", `${JSON.stringify(buildInfo, null, 2)}\n`, "utf8");
