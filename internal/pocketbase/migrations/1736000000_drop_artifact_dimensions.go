package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// init registers the migration that removes the pixel dimensions from
// the artifacts collection.
//
// The file name is the identity and the ordering key of the migration,
// so it sorts after the ones that are already applied.
func init() {
	registerMigration("1736000000_drop_artifact_dimensions.go", func(app core.App) error {
		return dropArtifactDimensions(app)
	})
}

// dropArtifactDimensions removes the width and the height columns from
// the artifacts collection.
//
// The columns held the pixel dimensions of an image artifact. The
// queue does not know what kind of work produced a file, so the
// dimensions were the one piece of domain knowledge inside it, and
// they were never read by anything: the interface shows the name, the
// media type and the size, and the dimensions stayed at zero for
// everything that was not an image.
//
// The dimensions belong in the metadata of the file that needs them,
// which is what the meta field is for.
func dropArtifactDimensions(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("artifacts")
	if err != nil {
		return err
	}

	// The collection is already there when the migration ran before.
	if collection.Fields.GetByName("width") == nil &&
		collection.Fields.GetByName("height") == nil {
		return nil
	}

	collection.Fields.RemoveByName("width")
	collection.Fields.RemoveByName("height")

	return app.Save(collection)
}
