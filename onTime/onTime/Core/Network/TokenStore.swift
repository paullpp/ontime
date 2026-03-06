import Foundation
import Security

actor TokenStore {
    static let shared = TokenStore()

    private let accessKey  = "com.paullpp.ontime.accessToken"
    private let refreshKey = "com.paullpp.ontime.refreshToken"
    private let userKey    = "com.paullpp.ontime.currentUser"

    var accessToken: String? { keychainRead(accessKey) }
    var refreshToken: String? { keychainRead(refreshKey) }

    func save(accessToken: String, refreshToken: String) {
        keychainWrite(accessKey, value: accessToken)
        keychainWrite(refreshKey, value: refreshToken)
    }

    func saveUser(_ user: User) {
        guard let data = try? JSONEncoder().encode(user) else { return }
        UserDefaults.standard.set(data, forKey: userKey)
    }

    func loadUser() -> User? {
        guard let data = UserDefaults.standard.data(forKey: userKey) else { return nil }
        return try? makeDecoder().decode(User.self, from: data)
    }

    func clearAll() {
        keychainDelete(accessKey)
        keychainDelete(refreshKey)
        UserDefaults.standard.removeObject(forKey: userKey)
    }

    // MARK: - Keychain

    private func keychainRead(_ key: String) -> String? {
        let query: [CFString: Any] = [
            kSecClass:       kSecClassGenericPassword,
            kSecAttrAccount: key,
            kSecReturnData:  true,
            kSecMatchLimit:  kSecMatchLimitOne
        ]
        var result: AnyObject?
        guard SecItemCopyMatching(query as CFDictionary, &result) == errSecSuccess,
              let data = result as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    private func keychainWrite(_ key: String, value: String) {
        guard let data = value.data(using: .utf8) else { return }
        let query:  [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrAccount: key]
        let update: [CFString: Any] = [kSecValueData: data]
        if SecItemUpdate(query as CFDictionary, update as CFDictionary) == errSecItemNotFound {
            var item = query; item[kSecValueData] = data
            SecItemAdd(item as CFDictionary, nil)
        }
    }

    private func keychainDelete(_ key: String) {
        let query: [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrAccount: key]
        SecItemDelete(query as CFDictionary)
    }
}
