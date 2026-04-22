package tools

// RegisterCoreTools registers all framework-level tools (no project-specific tools).
// This includes: shell, read_file, write_file, python, node, npm, npx, go_run,
// cypress, check_runtimes.
// Project-specific tools (e.g., oc_get, kafka_produce) are registered separately
// by each project.
func RegisterCoreTools(registry *Registry, constraints *ToolConstraints) {
	RegisterShellTools(registry, constraints)
	RegisterRuntimeTools(registry, constraints)
}
