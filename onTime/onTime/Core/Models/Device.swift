import Foundation

struct Device: Codable, Identifiable {
    let id: UUID
    let userId: UUID
    let apnsToken: String
    let isActive: Bool
    let createdAt: Date
    let updatedAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case userId = "user_id"
        case apnsToken = "apns_token"
        case isActive = "is_active"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}
