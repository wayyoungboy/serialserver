package database

import (
	"fmt"
	"os"
	"path/filepath"

	"vsp-server/internal/models"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化数据库
func Init(dbPath string) error {
	// 确保数据目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 自动迁移
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	return nil
}

// autoMigrate 自动迁移表结构
func autoMigrate() error {
	return DB.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.Device{},
		&models.Session{},
		&models.ConnectionLog{},
		&models.APIKey{},
	)
}

// Close 关闭数据库连接
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CreateDefaultData 创建默认数据
func CreateDefaultData() error {
	// 创建默认租户
	var tenantCount int64
	DB.Model(&models.Tenant{}).Count(&tenantCount)
	if tenantCount == 0 {
		tenant := &models.Tenant{
			Name:          "Default",
			Slug:          "default",
			Plan:          "free",
			MaxDevices:    10,
			MaxConnections: 20,
		}
		if err := DB.Create(tenant).Error; err != nil {
			return err
		}

		// 创建默认管理员
		admin := &models.User{
			Username:     "admin",
			Email:        "admin@vsp.local",
			PasswordHash: "$2a$10$Vh9iymyopLGcvM/Mrmi2zOQiE85sbn3Breh/wb75hwY0xKMX4rMEW", // password: admin123
			Role:         "admin",
			TenantID:     tenant.ID,
		}
		if err := DB.Create(admin).Error; err != nil {
			return err
		}
	}

	return nil
}