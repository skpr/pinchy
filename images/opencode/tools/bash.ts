import { tool } from "@opencode-ai/plugin"

export default tool({
  description: "Run a shell command in the remote Kubernetes environment",
  args: { command: tool.schema.string().describe("The shell command to run") },
  async execute(args, context) {
    // Mount and run inside the same directory the opencode session is using
    // so the remote environment operates on the session's workspace files.
    const result = await Bun.$`pinchy exec --session ${context.sessionID} --workdir ${context.directory} -- ${args.command}`
      .quiet()
      .nothrow()

    const stdout = result.stdout.toString()
    const stderr = result.stderr.toString()

    if (result.exitCode !== 0) {
      return `${stdout}${stderr ? `\nstderr:\n${stderr}` : ""}\n[exit code: ${result.exitCode}]`
    }

    return stdout
  },
})
