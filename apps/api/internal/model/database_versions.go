package model

// DBVersionInfo describes a supported database engine version.
type DBVersionInfo struct {
	Tag           string `json:"tag"`
	Label         string `json:"label"`
	IsRecommended bool   `json:"is_recommended"`
}

// SupportedVersions maps each database engine to its supported versions.
var SupportedVersions = map[DBEngine][]DBVersionInfo{
	DBPostgres: {
		{Tag: "18", Label: "PostgreSQL 18", IsRecommended: true},
		{Tag: "17", Label: "PostgreSQL 17", IsRecommended: false},
		{Tag: "16", Label: "PostgreSQL 16", IsRecommended: false},
		{Tag: "15", Label: "PostgreSQL 15", IsRecommended: false},
	},
	DBMySQL: {
		{Tag: "9.2", Label: "MySQL 9.2", IsRecommended: true},
		{Tag: "9.0", Label: "MySQL 9.0", IsRecommended: false},
		{Tag: "8.4", Label: "MySQL 8.4 LTS", IsRecommended: false},
		{Tag: "8.0", Label: "MySQL 8.0", IsRecommended: false},
	},
	DBRedis: {
		{Tag: "8.0", Label: "Redis 8.0", IsRecommended: true},
		{Tag: "7.4", Label: "Redis 7.4", IsRecommended: false},
		{Tag: "7.2", Label: "Redis 7.2", IsRecommended: false},
	},
	DBMongo: {
		{Tag: "8.0", Label: "MongoDB 8.0", IsRecommended: true},
		{Tag: "7.0", Label: "MongoDB 7.0", IsRecommended: false},
		{Tag: "6.0", Label: "MongoDB 6.0", IsRecommended: false},
	},
	DBMariaDB: {
		{Tag: "11.7", Label: "MariaDB 11.7", IsRecommended: true},
		{Tag: "11.4", Label: "MariaDB 11.4 LTS", IsRecommended: false},
		{Tag: "10.11", Label: "MariaDB 10.11 LTS", IsRecommended: false},
	},
}

// IsValidVersion returns true if the given tag is a supported version for the engine.
func IsValidVersion(engine DBEngine, tag string) bool {
	versions, ok := SupportedVersions[engine]
	if !ok {
		return false
	}
	for _, v := range versions {
		if v.Tag == tag {
			return true
		}
	}
	return false
}
