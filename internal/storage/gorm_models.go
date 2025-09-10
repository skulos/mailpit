package storage

// GormMailbox represents the mailbox table in the database
type GormMailbox struct {
	Created     int64  `gorm:"column:created;not null"`
	ID          string `gorm:"column:id;primaryKey;size:255"`
	MessageID   string `gorm:"column:message_id;size:255;not null"`
	Subject     string `gorm:"column:subject;type:text;not null"`
	Metadata    string `gorm:"column:metadata;type:text"`
	Size        int64  `gorm:"column:size;not null"`
	Inline      int    `gorm:"column:inline;not null"`
	Attachments int    `gorm:"column:attachments;not null"`
	Read        int    `gorm:"column:read"`
	Snippet     string `gorm:"column:snippet;type:text"`
	SearchText  string `gorm:"column:search_text;type:text"`
}

// TableName specifies the table name for GormMailbox
func (GormMailbox) TableName() string {
	return "mailbox"
}

// GormMailboxData represents the mailbox_data table in the database
type GormMailboxData struct {
	ID         string `gorm:"column:id;primaryKey;size:255"`
	Email      []byte `gorm:"column:email;type:bytea"`
	Compressed int    `gorm:"column:compressed;not null;default:0"`
}

// TableName specifies the table name for GormMailboxData
func (GormMailboxData) TableName() string {
	return "mailbox_data"
}

// GormTag represents the tags table in the database
type GormTag struct {
	ID   uint   `gorm:"column:id;primaryKey;autoIncrement"`
	Name string `gorm:"column:name;size:255;uniqueIndex;not null"`
}

// TableName specifies the table name for GormTag
func (GormTag) TableName() string {
	return "tags"
}

// GormMessageTag represents the message_tags table in the database
type GormMessageTag struct {
	Key   uint   `gorm:"column:key;primaryKey;autoIncrement"`
	ID    string `gorm:"column:id;size:255;not null"`
	TagID uint   `gorm:"column:tag_id;not null"`
}

// TableName specifies the table name for GormMessageTag
func (GormMessageTag) TableName() string {
	return "message_tags"
}

// GormSetting represents the settings table in the database
type GormSetting struct {
	Key   string `gorm:"column:key;primaryKey;size:255"`
	Value string `gorm:"column:value;type:text"`
}

// TableName specifies the table name for GormSetting
func (GormSetting) TableName() string {
	return "settings"
}

// GormSchema represents the schemas table in the database
type GormSchema struct {
	Version string `gorm:"column:version;primaryKey;size:50"`
}

// TableName specifies the table name for GormSchema
func (GormSchema) TableName() string {
	return "schemas"
}
