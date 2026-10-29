package pocketbase

// Collection names owned by this package.
//
// The settings store reads and writes app_settings, so this package is
// the only one that needs its name.
//
// The queue vocabulary -- the jobs and artifacts collections, the job
// statuses and the artifact kinds -- lives in the jobs package, which is
// the only consumer. The schema itself is created by the migration in
// internal/pocketbase/migrations, which names the collections it creates.
//
// An earlier version repeated the queue vocabulary here, with a comment
// claiming that importing it would create an import cycle. There was no
// cycle: this package imports nothing from the module except its own
// migrations. The duplicate is gone.
const (
	// CollectionAppSettings stores configuration and encrypted secrets.
	CollectionAppSettings = "app_settings"
)
