import Foundation

// MARK: - Request bodies

struct CreateTripRequest: Encodable {
    let originLat: Double
    let originLng: Double
    let originName: String
    let destinationLat: Double
    let destinationLng: Double
    let destinationName: String
    let desiredArrivalAt: Date
    let warningMinutes: Int

    enum CodingKeys: String, CodingKey {
        case originLat = "origin_lat"
        case originLng = "origin_lng"
        case originName = "origin_name"
        case destinationLat = "destination_lat"
        case destinationLng = "destination_lng"
        case destinationName = "destination_name"
        case desiredArrivalAt = "desired_arrival_at"
        case warningMinutes = "warning_minutes"
    }
}

struct UpdateTripRequest: Encodable {
    let desiredArrivalAt: Date
    let warningMinutes: Int

    enum CodingKeys: String, CodingKey {
        case desiredArrivalAt = "desired_arrival_at"
        case warningMinutes = "warning_minutes"
    }
}

// MARK: - Response bodies

struct AuthResponse: Decodable {
    let accessToken: String
    let refreshToken: String
    let expiresIn: Int
    let user: User?

    enum CodingKeys: String, CodingKey {
        case accessToken = "access_token"
        case refreshToken = "refresh_token"
        case expiresIn = "expires_in"
        case user
    }
}

// MARK: - Errors

enum APIError: LocalizedError {
    case unauthorized
    case notFound
    case serverError(Int, String?)
    case decodingError(Error)
    case networkError(Error)

    var errorDescription: String? {
        switch self {
        case .unauthorized:
            return "Session expired. Please sign in again."
        case .notFound:
            return "Not found."
        case .serverError(let code, let msg):
            return msg ?? "Server error (\(code))."
        case .decodingError(let e):
            return "Data error: \(e.localizedDescription)"
        case .networkError(let e):
            return e.localizedDescription
        }
    }
}
