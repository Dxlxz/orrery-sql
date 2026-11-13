package catalog

import "github.com/Dxlxz/orrery-sql/metadata"

// LoadMetadata adds the tables of a Bytebase schema snapshot to c.
func (c *Catalog) LoadMetadata(meta *metadata.DatabaseSchemaMetadata) {
	for _, s := range meta.GetSchemas() {
		for _, t := range s.GetTables() {
			if t.GetName() != "" {
				c.AddTable(t.GetName())
			}
		}
	}
}
