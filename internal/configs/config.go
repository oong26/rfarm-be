package configs

// Loader configuration to wrap the whole config piece
type Config struct {
	App      App      `yaml:"app"`
	Database Database `yaml:"database"`
	Redis    Redis    `yaml:"redis"`
	Auth     Auth     `yaml:"auth"`
	Cors     Cors     `yaml:"cors"`
	Log      Log      `yaml:"log"`
}

// App configuration struct
type App struct {
	Env             string `yaml:"env"`
	Addr            string `yaml:"addr"`
	ReadTimeout     string `yaml:"readTimeout"`
	WriteTimeout    string `yaml:"writeTimeout"`
	IdleTimeout     string `yaml:"idleTimeout"`
	ShutdownTimeout string `yaml:"shutdownTimeout"`
	DefaultTimezone string `yaml:"defaultTimezone"`
	MaxImportBytes  int64  `yaml:"maxImportBytes"`
}

// Database configuration struct
type Database struct {
	Url      string `yaml:"url" env:"DATABASE_URL"`
	MaxConns int16  `yaml:"maxConns"`
}

// Redis configuration struct
type Redis struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int16  `yaml:"db"`
	DialTimeout  string `yaml:"dialTimeout"`
	ReadTimeout  string `yaml:"readTimeout"`
	WriteTimeout string `yaml:"writeTimeout"`
	PoolSize     int16  `yaml:"poolSize"`
}

// Auth configuration struct
type Auth struct {
	JwtSigningKey   string `yaml:"jwtSigningKey" env:"JWT_SIGNING_KEY"`
	AccessTokenTTL  string `yaml:"accessTokenTTL"`
	RefreshTokenTTL string `yaml:"refreshTokenTTL"`
}

// Cors configuration struct
type Cors struct {
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

// Log configuration struct
type Log struct {
	Level string `yaml:"level"`
}
