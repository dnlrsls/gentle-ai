import { createHash, randomBytes } from "node:crypto"
import { constants } from "node:fs"
import { chmod, link, lstat, mkdir, open, rm, writeFile } from "node:fs/promises"
import { homedir } from "node:os"
import path from "node:path"

const LENSES = ["risk", "resilience", "readability", "reliability"]
const FINDING_FIELDS = ["location", "severity", "claim", "evidence_class", "causal_disposition", "proof_refs"]
const BINDING = /^GENTLE_AI_REVIEW_BINDING (\{[^\n]+\})(?:\n|$)/

function exactFields(value, expected, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(`${label} must be an object`)
  const actual = Object.keys(value).sort()
  const wanted = [...expected].sort()
  if (actual.length !== wanted.length || actual.some((key, index) => key !== wanted[index])) {
    throw new Error(`${label} contains missing or unknown fields`)
  }
}

export function canonicalReviewerResult(source) {
  let value
  try { value = JSON.parse(source) } catch { throw new Error("reviewer output must be one JSON value with no prose") }
  exactFields(value, ["findings", "evidence"], "reviewer result")
  if (!Array.isArray(value.findings) || !Array.isArray(value.evidence) || value.evidence.length === 0 ||
      value.evidence.some((item) => typeof item !== "string" || item.trim() === "")) {
    throw new Error("reviewer result requires findings and non-empty evidence arrays")
  }
  const findings = value.findings.map((finding, index) => {
    exactFields(finding, FINDING_FIELDS, `finding ${index + 1}`)
    if (!["BLOCKER", "CRITICAL", "WARNING", "SUGGESTION"].includes(finding.severity) ||
        !["deterministic", "inferential", "insufficient"].includes(finding.evidence_class) ||
        !["introduced", "behavior-activated", "worsened", "pre-existing", "base-only", "unknown"].includes(finding.causal_disposition) ||
        typeof finding.location !== "string" || typeof finding.claim !== "string" ||
        !Array.isArray(finding.proof_refs) || finding.proof_refs.some((proof) => typeof proof !== "string" || proof.trim() === "")) {
      throw new Error(`finding ${index + 1} is invalid`)
    }
    return {
      location: finding.location, severity: finding.severity, claim: finding.claim,
      evidence_class: finding.evidence_class, causal_disposition: finding.causal_disposition,
      proof_refs: finding.proof_refs,
    }
  })
  return `${JSON.stringify({ findings, evidence: value.evidence })}\n`
}

function parseBinding(args, lens) {
  const match = BINDING.exec(typeof args?.prompt === "string" ? args.prompt : "")
  if (!match) throw new Error("review task is missing GENTLE_AI_REVIEW_BINDING")
  let binding
  try { binding = JSON.parse(match[1]) } catch { throw new Error("review task binding is malformed") }
  exactFields(binding, ["lineage", "target", "lens"], "review task binding")
  if (typeof binding.lineage !== "string" || binding.lineage.length > 128 || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(binding.lineage) ||
      !/^sha256:[a-f0-9]{64}$/.test(binding.target) || binding.lens !== lens) {
    throw new Error("review task binding does not match the selected lens")
  }
  return binding
}

async function safeDirectory(directory) {
  await mkdir(directory, { recursive: true, mode: 0o700 })
  let current = path.parse(directory).root
  for (const segment of directory.slice(current.length).split(path.sep).filter(Boolean)) {
    current = path.join(current, segment)
    const info = await lstat(current)
    if (info.isSymbolicLink() || !info.isDirectory()) throw new Error(`unsafe review artifact directory: ${current}`)
  }
}

export async function persistReviewerResult(root, binding, source, afterWrite) {
  const lens = binding.lens
  const order = LENSES.indexOf(lens)
  if (order < 0) throw new Error(`unsupported review lens ${lens}`)
  const directory = path.join(root, binding.lineage, binding.target.slice(7))
  await safeDirectory(directory)
  const finalPath = path.join(directory, `${String(order).padStart(2, "0")}-${lens}.json`)
  try { await lstat(finalPath); throw new Error(`duplicate review result for ${lens}`) } catch (error) {
    if (error?.code !== "ENOENT") throw error
  }
  const canonical = canonicalReviewerResult(source)
  const temporary = path.join(directory, `.${lens}.${randomBytes(8).toString("hex")}.tmp`)
  try {
    await writeFile(temporary, canonical, { flag: "wx", mode: 0o600 })
    await chmod(temporary, 0o600)
    await link(temporary, finalPath)
  } finally {
    await rm(temporary, { force: true })
  }
  if (afterWrite) await afterWrite(finalPath)
  const pathInfo = await lstat(finalPath)
  if (!pathInfo.isFile() || pathInfo.isSymbolicLink()) throw new Error("review result readback is not a regular file")
  const handle = await open(finalPath, constants.O_RDONLY | (constants.O_NOFOLLOW || 0))
  let readback
  try {
    const info = await handle.stat()
    if (!info.isFile() || process.platform !== "win32" && (info.mode & 0o077) !== 0) throw new Error("review result permissions or file type changed")
    readback = await handle.readFile()
  } finally {
    await handle.close()
  }
  const expected = createHash("sha256").update(canonical).digest("hex")
  const actual = createHash("sha256").update(readback).digest("hex")
  if (actual !== expected) throw new Error("review result readback hash mismatch")
  return { path: finalPath, sha256: `sha256:${actual}`, lens, lineage: binding.lineage, target: binding.target, order }
}

export const ReviewResultArtifactsPlugin = async () => ({
  "tool.execute.after": async (input, output) => {
    if (input.tool !== "task" || typeof input.args?.subagent_type !== "string" || !input.args.subagent_type.startsWith("review-")) return
    const lens = input.args.subagent_type.slice("review-".length)
    if (!LENSES.includes(lens)) return
    const binding = parseBinding(input.args, lens)
    const root = process.env.GENTLE_AI_REVIEW_RESULT_ROOT || path.join(homedir(), ".gentle-ai", "review-results")
    const metadata = await persistReviewerResult(root, binding, output.output)
    output.output = JSON.stringify({ schema: "gentle-ai.review-result-artifact/v1", ...metadata })
  },
})

export default ReviewResultArtifactsPlugin
