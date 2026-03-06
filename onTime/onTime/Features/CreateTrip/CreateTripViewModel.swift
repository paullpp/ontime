import Foundation
import MapKit

@Observable
final class CreateTripViewModel {
    var originItem: MKMapItem?
    var destinationItem: MKMapItem?
    var desiredArrivalAt: Date = Calendar.current.date(byAdding: .hour, value: 2, to: Date()) ?? Date()
    var warningMinutes: Int = 0

    var isLoading = false
    var errorMessage: String?

    var canCreate: Bool {
        destinationItem != nil && desiredArrivalAt > Date()
    }

    var originName: String {
        originItem?.name ?? originItem?.placemark.title ?? "Current location"
    }

    var destinationName: String {
        destinationItem?.name ?? destinationItem?.placemark.title ?? ""
    }

    func prefillOrigin(from locationManager: LocationManager) {
        guard originItem == nil,
              let location = locationManager.currentLocation else { return }
        let placemark = MKPlacemark(coordinate: location.coordinate)
        let item = MKMapItem(placemark: placemark)
        item.name = "Current Location"
        originItem = item
    }

    @MainActor
    func create() async throws -> Trip {
        guard let destItem = destinationItem else {
            throw AppError("Destination is required.")
        }

        isLoading = true
        defer { isLoading = false }

        let coord = destItem.placemark.coordinate
        let originCoord = originItem?.placemark.coordinate

        let req = CreateTripRequest(
            originLat:        originCoord?.latitude  ?? 0,
            originLng:        originCoord?.longitude ?? 0,
            originName:       originName,
            destinationLat:   coord.latitude,
            destinationLng:   coord.longitude,
            destinationName:  destinationName,
            desiredArrivalAt: desiredArrivalAt,
            warningMinutes:   warningMinutes
        )
        return try await APIClient.shared.createTrip(req)
    }
}

struct AppError: LocalizedError {
    let errorDescription: String?
    init(_ message: String) { errorDescription = message }
}
