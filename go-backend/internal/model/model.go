// Package model 定义数据模型与 DTO。
package model

// Group 数据源分组（树形，parent_id=0 为顶级）。
type Group struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	ParentID    int64    `json:"parent_id"`
	SortOrder   int      `json:"sort_order"`
	Description string   `json:"description"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Children    []*Group `json:"children,omitempty"`
}

// DataSource 数据源主记录（密文字段不出现在 JSON 中）。
type DataSource struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	GroupID  int64  `json:"group_id"`
	DbType   string `json:"db_type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DbName   string `json:"db_name"`
	Username string `json:"username"`

	PasswordEnc []byte `json:"-"`
	PasswordIV  []byte `json:"-"`

	ExtraParams string `json:"extra_params"`

	TunnelType       string `json:"tunnel_type"`
	TunnelHost       string `json:"tunnel_host"`
	TunnelPort       int    `json:"tunnel_port"`
	TunnelUser       string `json:"tunnel_user"`
	TunnelAuthScheme string `json:"tunnel_auth_scheme"`
	TunnelTransport  string `json:"tunnel_transport"`
	TunnelUseSSL     int    `json:"tunnel_use_ssl"`

	TunnelPasswordEnc []byte `json:"-"`
	TunnelPasswordIV  []byte `json:"-"`
	TunnelKeyEnc      []byte `json:"-"`
	TunnelKeyIV       []byte `json:"-"`
	TunnelKeyPassEnc  []byte `json:"-"`
	TunnelKeyPassIV   []byte `json:"-"`

	Status     int     `json:"status"` // 0=未测 1=正常 2=异常
	LastTestAt *string `json:"last_test_at"`
	LastError  string  `json:"last_error"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// DataSourceInput 新建/编辑数据源请求体（含明文凭据，仅存在于请求生命周期内）。
type DataSourceInput struct {
	Name     string `json:"name"`
	GroupID  int64  `json:"group_id"`
	DbType   string `json:"db_type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DbName   string `json:"db_name"`
	Username string `json:"username"`
	Password string `json:"password"`

	ExtraParams string `json:"extra_params"`

	TunnelType         string `json:"tunnel_type"`
	TunnelHost         string `json:"tunnel_host"`
	TunnelPort         int    `json:"tunnel_port"`
	TunnelUser         string `json:"tunnel_user"`
	TunnelPassword     string `json:"tunnel_password"`
	TunnelPrivateKey   string `json:"tunnel_private_key"`
	TunnelKeyPassphrase string `json:"tunnel_key_passphrase"`
	TunnelAuthScheme   string `json:"tunnel_auth_scheme"`
	TunnelTransport    string `json:"tunnel_transport"`
	TunnelUseSSL       bool   `json:"tunnel_use_ssl"`
}

// Driver 驱动元数据。
type Driver struct {
	ID               int64  `json:"id"`
	DbType           string `json:"db_type"`
	DriverKind       string `json:"driver_kind"` // python | jdbc
	PackageName      string `json:"package_name"`
	Version          string `json:"version"`
	DriverClass      string `json:"driver_class"`
	JarFilename      string `json:"jar_filename"`
	JarPath          string `json:"jar_path"`
	FileSize         int64  `json:"file_size"`
	IsActive         int    `json:"is_active"`
	Installed        int    `json:"installed"`
	InstalledVersion string `json:"installed_version"`
	Note             string `json:"note"`
	UploadedAt       string `json:"uploaded_at"`
	UpdatedAt        string `json:"updated_at"`
}

// DbTypeCustom 自定义数据库类型。
type DbTypeCustom struct {
	DbType          string `json:"key"`
	NameZh          string `json:"name_zh"`
	NameEn          string `json:"name_en"`
	DriverClassHint string `json:"driver_class_hint"`
	IsJdbc          int    `json:"is_jdbc"`
	CreatedAt       string `json:"created_at"`
}

// DbTypeView /db-types 接口返回视图（内置 + 自定义 - 隐藏）。
type DbTypeView struct {
	Key             string `json:"key"`
	NameZh          string `json:"name_zh"`
	NameEn          string `json:"name_en"`
	DriverClassHint string `json:"driver_class_hint"`
	IsJdbc          bool   `json:"is_jdbc"`
	Order           int    `json:"order"`
	IsCustom        bool   `json:"is_custom"`
	DefaultPort     int    `json:"default_port"`
	Hidden          bool   `json:"hidden"`
}

// ExportRow 导出 CSV 使用的行（不含密码）。
type ExportRow struct {
	DbType       string
	Name         string
	Host         string
	Port         int
	DbName       string
	Username     string
	GroupName    string
	TunnelType   string
	TunnelHost   string
	TunnelPort   int
	TunnelUser   string
}
