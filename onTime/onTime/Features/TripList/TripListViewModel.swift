import Foundation

@Observable
final class TripListViewModel {
    var trips: [Trip] = []
    var isLoading = false
    var errorMessage: String?

    @MainActor
    func load() async {
        isLoading = true
        errorMessage = nil
        do {
            trips = try await APIClient.shared.listTrips()
        } catch APIError.unauthorized {
            // Coordinator will handle sign-out upstream via the error propagation.
        } catch {
            errorMessage = error.localizedDescription
        }
        isLoading = false
    }

    @MainActor
    func cancel(_ trip: Trip) async {
        do {
            try await APIClient.shared.cancelTrip(id: trip.id)
            trips.removeAll { $0.id == trip.id }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    @MainActor
    func refresh(trip updatedTrip: Trip) {
        if let idx = trips.firstIndex(where: { $0.id == updatedTrip.id }) {
            trips[idx] = updatedTrip
        }
    }

    @MainActor
    func append(_ trip: Trip) {
        trips.insert(trip, at: 0)
    }
}
