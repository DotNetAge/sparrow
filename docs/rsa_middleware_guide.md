# RSA 中间件使用指南

## 1. 简介

RSA 中间件是一个基于 RSA PSS 签名验证的 HTTP 认证中间件，用于保护 API 接口免受未授权访问和请求篡改。该中间件提供了灵活的配置选项和可扩展的接口，适用于各种安全认证场景。

## 2. 功能特点

- **RSA PSS 签名验证**：使用 RSA PSS 算法验证请求签名，确保请求的真实性和完整性
- **防重放攻击**：通过 nonce 验证和时间窗口机制防止重放攻击
- **请求体验证**：签名包含请求体，防止请求内容被篡改
- **灵活的配置**：支持自定义请求头、时间窗口和组件替换
- **性能优化**：内置签名缓存机制，减少重复的密码学操作
- **可扩展接口**：提供非对称密钥管理、nonce 存储和签名缓存的接口，支持自定义实现

## 3. 核心接口

### 3.1 NonceStore 接口

用于存储和验证 nonce 值，防止重放攻击：

```go
type NonceStore interface {
    Set(nonce string, expiration time.Time) error
    Exists(nonce string) (bool, error)
}
```

### 3.2 PublicKeyProvider 接口

动态提供 RSA 公钥，支持基于客户端 ID 的密钥管理：

```go
type PublicKeyProvider interface {
    GetPublicKey(clientID string) (*rsa.PublicKey, error)
}
```

### 3.3 SignatureCache 接口

缓存签名验证结果，提高性能：

```go
type SignatureCache interface {
    Set(key string, value bool, expiration time.Time) error
    Get(key string) (bool, error)
}
```

## 4. 配置选项

### 4.1 RSAAuthConfig 结构

```go
type RSAAuthConfig struct {
    ClientIDHeader         string        // 客户端 ID 头
    ClientSignatureHeader  string        // 签名头
    NonceHeader            string        // Nonce 头
    TimestampHeader        string        // 时间戳头
    TimeWindow             int64         // 时间窗口（秒）
    NonceStore             NonceStore    // Nonce 存储
    SignatureCache         SignatureCache // 签名缓存
    PublicKeyProvider      PublicKeyProvider // 公钥提供者
}
```

### 4.2 默认配置

- ClientIDHeader: "X-Client-ID"
- ClientSignatureHeader: "X-Client-Signature"
- NonceHeader: "X-Client-Nonce"
- TimestampHeader: "X-Client-Timestamp"
- TimeWindow: 300 (5分钟)
- NonceStore: 内存实现
- SignatureCache: 内存实现
- PublicKeyProvider: 内存实现

## 5. 使用示例

### 5.1 基本使用

```go
import (
    "github.com/DotNetAge/sparrow/pkg/auth"
    "github.com/DotNetAge/sparrow/pkg/adapter/http/middlewares"
)

// 创建默认配置
config := auth.NewRSAAuthConfig()

// 添加默认公钥
config.PublicKeyProvider.(*auth.MemoryPublicKeyProvider).AddPublicKey("client1", publicKey)

// 创建中间件
rsaMiddleware := middlewares.ConfigurableRSAClientAuthMiddleware(config)

// 在路由中使用
router := gin.Default()
router.Use(rsaMiddleware)
```

### 5.2 自定义配置

```go
import (
    "time"
    "github.com/DotNetAge/sparrow/pkg/auth"
    "github.com/DotNetAge/sparrow/pkg/adapter/http/middlewares"
)

// 创建自定义配置
config := &auth.RSAAuthConfig{
    ClientIDHeader:         "X-App-ID",
    ClientSignatureHeader:  "X-App-Signature",
    NonceHeader:            "X-App-Nonce",
    TimestampHeader:        "X-App-Timestamp",
    TimeWindow:             600, // 10分钟
    NonceStore:             auth.NewMemoryNonceStore(),
    SignatureCache:         auth.NewMemorySignatureCache(),
    PublicKeyProvider:      auth.NewMemoryPublicKeyProvider(),
}

// 添加公钥
config.PublicKeyProvider.(*auth.MemoryPublicKeyProvider).AddPublicKey("app1", publicKey)

// 创建中间件
rsaMiddleware := middlewares.ConfigurableRSAClientAuthMiddleware(config)

// 在路由中使用
router := gin.Default()
apiGroup := router.Group("/api")
apiGroup.Use(rsaMiddleware)
{
    apiGroup.GET("/resource", handler)
}
```

### 5.3 自定义组件实现

#### 5.3.1 自定义 PublicKeyProvider

```go
import (
    "crypto/rsa"
    "github.com/DotNetAge/sparrow/pkg/auth"
)

type DBPublicKeyProvider struct {
    // 数据库连接等
}

func (p *DBPublicKeyProvider) GetPublicKey(clientID string) (*rsa.PublicKey, error) {
    // 从数据库获取公钥
    // ...
    return publicKey, nil
}

// 使用自定义 PublicKeyProvider
config := auth.NewRSAAuthConfig()
config.PublicKeyProvider = &DBPublicKeyProvider{}
```

#### 5.3.2 自定义 NonceStore

```go
import (
    "time"
    "github.com/DotNetAge/sparrow/pkg/auth"
)

type RedisNonceStore struct {
    // Redis 客户端等
}

func (s *RedisNonceStore) Set(nonce string, expiration time.Time) error {
    // 存储到 Redis
    // ...
    return nil
}

func (s *RedisNonceStore) Exists(nonce string) (bool, error) {
    // 从 Redis 检查
    // ...
    return exists, nil
}

// 使用自定义 NonceStore
config := auth.NewRSAAuthConfig()
config.NonceStore = &RedisNonceStore{}
```

## 6. 签名生成

客户端需要生成符合要求的签名才能通过验证。签名生成步骤如下：

1. **准备签名材料**：
   - 客户端 ID
   - 当前时间戳（Unix 秒）
   - 随机 nonce 值
   - 请求方法（如 GET, POST）
   - 请求路径
   - 请求体（如果有）

2. **构建签名字符串**：
   ```
   clientID + "|" + timestamp + "|" + nonce + "|" + method + "|" + path + "|" + body
   ```

3. **使用私钥签名**：
   - 使用 RSA PSS 算法
   - 哈希函数：SHA-256
   - 盐长度：32 字节

4. **添加请求头**：
   - X-Client-ID: 客户端 ID
   - X-Client-Signature: Base64 编码的签名
   - X-Client-Nonce: 随机 nonce 值
   - X-Client-Timestamp: 当前时间戳

### 6.1 Go 客户端示例

```go
import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
    "net/http"
    "strings"
    "time"
)

func signRequest(privateKey *rsa.PrivateKey, clientID, method, path, body string) (map[string]string, error) {
    // 生成 nonce
    nonce := generateNonce()
    
    // 获取时间戳
    timestamp := fmt.Sprintf("%d", time.Now().Unix())
    
    // 构建签名字符串
    signData := strings.Join([]string{
        clientID,
        timestamp,
        nonce,
        method,
        path,
        body,
    }, "|")
    
    // 签名
    hashed := sha256.Sum256([]byte(signData))
    signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hashed[:], nil)
    if err != nil {
        return nil, err
    }
    
    // 编码签名
    encodedSignature := base64.StdEncoding.EncodeToString(signature)
    
    // 返回请求头
    return map[string]string{
        "X-Client-ID":         clientID,
        "X-Client-Signature":  encodedSignature,
        "X-Client-Nonce":      nonce,
        "X-Client-Timestamp":  timestamp,
    }, nil
}

func generateNonce() string {
    // 生成随机 nonce
    // ...
    return "random-nonce"
}
```

### 6.2 Swift (Apple 客户端) 示例

```swift
import Foundation
import CommonCrypto

enum RSAError: Error {
    case invalidPrivateKey
    case signingFailed
    case encodingFailed
}

extension Data {
    func base64EncodedStringUrlSafe() -> String {
        return base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

extension String {
    func sha256() -> Data {
        let data = self.data(using: .utf8)!
        var hash = [UInt8](repeating: 0, count: Int(CC_SHA256_DIGEST_LENGTH))
        data.withUnsafeBytes { 
            _ = CC_SHA256($0.baseAddress, CC_LONG(data.count), &hash)
        }
        return Data(hash)
    }
}

class RSASigner {
    private let privateKey: SecKey
    
    init(privateKey: SecKey) {
        self.privateKey = privateKey
    }
    
    func signRequest(clientID: String, method: String, path: String, body: String) throws -> [String: String] {
        // 生成 nonce
        let nonce = generateNonce()
        
        // 获取时间戳
        let timestamp = String(Int(Date().timeIntervalSince1970))
        
        // 构建签名字符串
        let signData = [clientID, timestamp, nonce, method, path, body].joined(separator: "|")
        
        // 签名
        guard let signature = sign(data: signData) else {
            throw RSAError.signingFailed
        }
        
        // 编码签名
        let encodedSignature = signature.base64EncodedStringUrlSafe()
        
        // 返回请求头
        return [
            "X-Client-ID": clientID,
            "X-Client-Signature": encodedSignature,
            "X-Client-Nonce": nonce,
            "X-Client-Timestamp": timestamp
        ]
    }
    
    private func sign(data: String) -> Data? {
        let digest = data.sha256()
        var error: Unmanaged<CFError>?
        
        guard let signature = SecKeyCreateSignature(
            privateKey,
            .rsaSignatureMessagePSSSHA256,
            digest as CFData,
            &error
        ) as Data? else {
            print("Signing error: \(error?.takeRetainedValue() ?? "Unknown error" as CFError)")
            return nil
        }
        
        return signature
    }
    
    private func generateNonce() -> String {
        let length = 32
        let charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
        return String((0..<length).map { _ in charset.randomElement()! })
    }
}

// 使用示例
func sendAuthenticatedRequest() {
    // 加载私钥
    guard let privateKey = loadPrivateKey() else {
        print("Failed to load private key")
        return
    }
    
    let signer = RSASigner(privateKey: privateKey)
    
    do {
        // 准备请求数据
        let clientID = "apple-client-1"
        let method = "POST"
        let path = "/api/resource"
        let body = "{\"key\": \"value\"}"
        
        // 生成签名和请求头
        let headers = try signer.signRequest(clientID: clientID, method: method, path: path, body: body)
        
        // 创建 URLRequest
        guard let url = URL(string: "https://api.example.com" + path) else {
            return
        }
        
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.httpBody = body.data(using: .utf8)
        
        // 添加请求头
        for (key, value) in headers {
            request.setValue(value, forHTTPHeaderField: key)
        }
        
        // 发送请求
        let task = URLSession.shared.dataTask(with: request) { data, response, error in
            // 处理响应
            if let error = error {
                print("Error: \(error)")
                return
            }
            
            if let data = data {
                print("Response: \(String(data: data, encoding: .utf8) ?? "")")
            }
        }
        
        task.resume()
        
    } catch {
        print("Error signing request: \(error)")
    }
}

func loadPrivateKey() -> SecKey? {
    // 从文件或钥匙串加载私钥
    // 这里是示例代码，实际实现需要根据私钥存储方式调整
    // ...
    return nil
}
```

### 6.3 Android 客户端示例

```java
import android.util.Base64;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.HashMap;
import java.util.Map;
import java.util.Random;
import okhttp3.*;
import okhttp3.MediaType;
import okhttp3.Request;
import okhttp3.RequestBody;

public class RSASigner {
    private final PrivateKey privateKey;
    
    public RSASigner(PrivateKey privateKey) {
        this.privateKey = privateKey;
    }
    
    public Map<String, String> signRequest(String clientID, String method, String path, String body) throws Exception {
        // 生成 nonce
        String nonce = generateNonce();
        
        // 获取时间戳
        String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
        
        // 构建签名字符串
        String signData = clientID + "|" + timestamp + "|" + nonce + "|" + method + "|" + path + "|" + body;
        
        // 签名
        byte[] signature = sign(signData.getBytes(StandardCharsets.UTF_8));
        
        // 编码签名
        String encodedSignature = Base64.encodeToString(signature, Base64.NO_WRAP);
        
        // 返回请求头
        Map<String, String> headers = new HashMap<>();
        headers.put("X-Client-ID", clientID);
        headers.put("X-Client-Signature", encodedSignature);
        headers.put("X-Client-Nonce", nonce);
        headers.put("X-Client-Timestamp", timestamp);
        
        return headers;
    }
    
    private byte[] sign(byte[] data) throws Exception {
        Signature signature = Signature.getInstance("SHA256withRSA/PSS");
        signature.initSign(privateKey);
        signature.update(data);
        return signature.sign();
    }
    
    private String generateNonce() {
        int length = 32;
        String charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
        Random random = new Random();
        StringBuilder sb = new StringBuilder(length);
        for (int i = 0; i < length; i++) {
            sb.append(charset.charAt(random.nextInt(charset.length())));
        }
        return sb.toString();
    }
    
    public static PrivateKey loadPrivateKey(String privateKeyPEM) throws Exception {
        // 移除 PEM 头部和尾部
        String privateKeyContent = privateKeyPEM
                .replace("-----BEGIN PRIVATE KEY-----", "")
                .replace("-----END PRIVATE KEY-----", "")
                .replaceAll("\\s", "");
        
        // 解码 Base64
        byte[] privateKeyBytes = Base64.decode(privateKeyContent, Base64.DEFAULT);
        
        // 生成私钥
        PKCS8EncodedKeySpec keySpec = new PKCS8EncodedKeySpec(privateKeyBytes);
        KeyFactory keyFactory = KeyFactory.getInstance("RSA");
        return keyFactory.generatePrivate(keySpec);
    }
}

// 使用示例
public class ApiClient {
    public void sendAuthenticatedRequest() {
        try {
            // 加载私钥
            String privateKeyPEM = "-----BEGIN PRIVATE KEY-----...-----END PRIVATE KEY-----";
            PrivateKey privateKey = RSASigner.loadPrivateKey(privateKeyPEM);
            
            RSASigner signer = new RSASigner(privateKey);
            
            // 准备请求数据
            String clientID = "android-client-1";
            String method = "POST";
            String path = "/api/resource";
            String body = "{\"key\": \"value\"}";
            
            // 生成签名和请求头
            Map<String, String> headers = signer.signRequest(clientID, method, path, body);
            
            // 创建和发送请求（使用 OkHttp 示例）
            OkHttpClient client = new OkHttpClient();
            
            RequestBody requestBody = RequestBody.create(
                    body, MediaType.parse("application/json; charset=utf-8"));
            
            Request.Builder requestBuilder = new Request.Builder()
                    .url("https://api.example.com" + path)
                    .method(method, requestBody);
            
            // 添加请求头
            for (Map.Entry<String, String> entry : headers.entrySet()) {
                requestBuilder.addHeader(entry.getKey(), entry.getValue());
            }
            
            Request request = requestBuilder.build();
            
            client.newCall(request).enqueue(new Callback() {
                @Override
                public void onFailure(Call call, IOException e) {
                    e.printStackTrace();
                }
                
                @Override
                public void onResponse(Call call, Response response) throws IOException {
                    if (response.isSuccessful()) {
                        String responseBody = response.body().string();
                        System.out.println("Response: " + responseBody);
                    } else {
                        System.out.println("Error: " + response.code());
                    }
                }
            });
            
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}
```

## 7. 安全注意事项

1. **密钥管理**：
   - 私钥必须安全存储，不得泄露
   - 定期轮换密钥对
   - 公钥分发渠道必须安全

2. **Nonce 管理**：
   - Nonce 值必须足够随机
   - 生产环境建议使用分布式存储（如 Redis）作为 NonceStore
   - 确保 Nonce 有适当的过期时间

3. **时间同步**：
   - 客户端和服务器的时间必须同步
   - 建议使用 NTP 服务保持时间同步
   - 时间窗口不宜设置过大，默认 5 分钟即可

4. **性能优化**：
   - 生产环境建议使用分布式缓存（如 Redis）作为 SignatureCache
   - 合理设置缓存过期时间

5. **错误处理**：
   - 不要在错误响应中泄露敏感信息
   - 建议使用统一的错误码和错误信息

## 8. 测试

中间件包含完整的单元测试，覆盖了以下场景：

- 正常认证流程
- 缺失必要请求头
- 无效的客户端 ID
- 过期的时间戳
- 重放攻击
- 无效的签名

运行测试：

```bash
go test -v ./pkg/adapter/http/middlewares/
```

## 9. 故障排除

### 9.1 常见错误

| 错误信息                  | 可能原因               | 解决方案                           |
| ------------------------- | ---------------------- | ---------------------------------- |
| "missing required header" | 缺少必要的请求头       | 确保客户端发送了所有必需的请求头   |
| "invalid client ID"       | 客户端 ID 不存在或无效 | 检查客户端 ID 是否正确注册         |
| "expired timestamp"       | 时间戳过期             | 检查客户端和服务器的时间同步状态   |
| "nonce already exists"    | Nonce 已被使用         | 确保客户端每次请求使用不同的 nonce |
| "invalid signature"       | 签名验证失败           | 检查签名生成算法和私钥是否正确     |

### 9.2 调试建议

- 启用详细日志，查看中间件的验证过程
- 使用测试工具（如 Postman）手动构造请求并测试
- 检查客户端和服务器的时间差
- 验证公钥和私钥是否匹配

## 10. 小结

RSA 中间件提供了一种安全、灵活的 API 认证方案，通过 RSA PSS 签名验证确保请求的真实性和完整性，同时通过防重放攻击机制提高了系统的安全性。该中间件的可扩展设计使其适用于各种复杂的认证场景，是构建安全 API 的理想选择。