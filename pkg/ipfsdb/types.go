package ipfsdb

// Table represents a table in the database
type Table struct {
	SchemaCID string            `json:"schema_cid"`
	Rows      []string          `json:"rows"`      // CIDs of row objects
	Indexes   map[string]string `json:"indexes,omitempty"`   // map[column_name]index_cid
}

// Database represents the entire database
type Database struct {
	Tables map[string]string `json:"tables"` // map[table_name]table_cid
}

// RegistryEntry defines the structure for an entry in registry.json
type RegistryEntry struct {
	DbName    string `json:"db_name"`
	ProgramID string `json:"program_id"` // The IPNS Name (Key ID)
	KeyName   string `json:"key_name"`   // The local key name
}
