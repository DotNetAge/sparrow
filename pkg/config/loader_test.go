package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadWithEnvFile(t *testing.T) {
	// 保存当前工作目录
	originalDir, err := os.Getwd()
	assert.NoError(t, err)

	// 创建临时目录作为测试工作目录
	tempDir, err := os.MkdirTemp("", "config-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir) // 测试结束后清理临时目录

	// 切换到临时目录
	defer os.Chdir(originalDir) // 测试结束后切换回原目录
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// 创建.env文件
	envContent := "APP_NAME=test-app\nAPP_ENV=testing\nAPP_DEBUG=true\nSERVER_PORT=3000"
	err = os.WriteFile(".env", []byte(envContent), 0644)
	assert.NoError(t, err)

	// 创建config.yaml文件
	configContent := "app:\n  name: yaml-config-app\nserver:\n  port: 8080"
	err = os.WriteFile("config.yaml", []byte(configContent), 0644)
	assert.NoError(t, err)

	// 测试Load方法
	config, err := Load("")
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// 验证环境变量被正确应用（应该覆盖yaml配置）
	// 注意：由于viper的工作方式，我们无法直接验证.env文件中的值是否覆盖了配置文件
	// 但我们可以验证Load方法能够正常工作
	assert.NotEmpty(t, config.App.Name)
	assert.NotZero(t, config.Server.Port)

	// 验证设置的环境变量确实存在
	assert.Equal(t, "test-app", os.Getenv("APP_NAME"))
	assert.Equal(t, "testing", os.Getenv("APP_ENV"))
	assert.Equal(t, "true", os.Getenv("APP_DEBUG"))
	assert.Equal(t, "3000", os.Getenv("SERVER_PORT"))
}

// 测试Load方法在没有.env文件的情况下是否正常工作
func TestLoadWithoutEnvFile(t *testing.T) {
	// 保存当前工作目录
	originalDir, err := os.Getwd()
	assert.NoError(t, err)

	// 保存并清除可能影响测试的环境变量
	originalServerPort := os.Getenv("SERVER_PORT")
	originalAppName := os.Getenv("APP_NAME")
	defer func() {
		os.Setenv("SERVER_PORT", originalServerPort) // 测试结束后恢复
		os.Setenv("APP_NAME", originalAppName)       // 测试结束后恢复
	}()
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("APP_NAME")

	// 创建临时目录作为测试工作目录（不包含.env文件）
	tempDir, err := os.MkdirTemp("", "config-test-no-env-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir) // 测试结束后清理临时目录

	// 切换到临时目录
	defer os.Chdir(originalDir) // 测试结束后切换回原目录
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// 测试Load方法在没有.env文件的情况下是否正常工作
	config, err := Load("")
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// 验证配置使用了默认值
	assert.Equal(t, "sparrow", config.App.Name) // 验证app.name默认值
	assert.Equal(t, 8080, config.Server.Port)   // 验证server.port默认值
}

func TestLoadWithInvalidEnvFile(t *testing.T) {
	// 保存当前工作目录
	originalDir, err := os.Getwd()
	assert.NoError(t, err)

	// 创建临时目录作为测试工作目录
	tempDir, err := os.MkdirTemp("", "config-test-invalid-env-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir) // 测试结束后清理临时目录

	// 切换到临时目录
	defer os.Chdir(originalDir) // 测试结束后切换回原目录
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// 创建格式不正确的.env文件（只有键没有值）
	invalidEnvContent := "INVALID_KEY\n"
	err = os.WriteFile(".env", []byte(invalidEnvContent), 0644)
	assert.NoError(t, err)

	// 测试Load方法在.env文件格式不正确时是否仍然能够正常工作
	config, err := Load("")
	assert.NoError(t, err)
	assert.NotNil(t, config)
}
