import Foundation
import CoreLocation

struct Trip: Codable, Identifiable, Hashable {
    let id: UUID
    let userId: UUID
    let originLat: Double
    let originLng: Double
    let originName: String
    let destinationLat: Double
    let destinationLng: Double
    let destinationName: String
    let desiredArrivalAt: Date
    let warningMinutes: Int
    let status: Status
    let latestEtaSeconds: Int
    let nextPollAt: Date
    let notificationSentAt: Date?
    let createdAt: Date
    let updatedAt: Date

    enum Status: String, Codable {
        case active, notified, cancelled, expired
    }

    var isActive: Bool { status == .active }

    var shouldLeaveAt: Date {
        desiredArrivalAt.addingTimeInterval(-Double(latestEtaSeconds + warningMinutes * 60))
    }

    var origin: CLLocationCoordinate2D {
        CLLocationCoordinate2D(latitude: originLat, longitude: originLng)
    }

    var destination: CLLocationCoordinate2D {
        CLLocationCoordinate2D(latitude: destinationLat, longitude: destinationLng)
    }

    var hasOrigin: Bool { originLat != 0 || originLng != 0 }

    enum CodingKeys: String, CodingKey {
        case id, status
        case userId = "user_id"
        case originLat = "origin_lat"
        case originLng = "origin_lng"
        case originName = "origin_name"
        case destinationLat = "destination_lat"
        case destinationLng = "destination_lng"
        case destinationName = "destination_name"
        case desiredArrivalAt = "desired_arrival_at"
        case warningMinutes = "warning_minutes"
        case latestEtaSeconds = "latest_eta_seconds"
        case nextPollAt = "next_poll_at"
        case notificationSentAt = "notification_sent_at"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}
