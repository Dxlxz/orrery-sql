package catalog

import "github.com/Dxlxz/orrery-sql/metadata"

// LoadMetadata adds the collections of a Bytebase schema snapshot to c. Sync
// reports each collection as a table.
func (c *Catalog) LoadMetadata(meta *metadata.DatabaseSchemaMetadata) {
	for _, s := range meta.GetSchemas() {
		for _, t := range s.GetTables() {
			if t.GetName() != "" {
				c.AddCollection(t.GetName())
			}
		}
	}
}
