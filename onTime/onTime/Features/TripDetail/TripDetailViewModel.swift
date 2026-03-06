import Foundation

@Observable
final class TripDetailViewModel {
    var trip: Trip
    var isLoading = false
    var errorMessage: String?
    var showEditSheet = false

    init(trip: Trip) {
        self.trip = trip
    }

    @MainActor
    func refresh() async {
        isLoading = true
        do {
            trip = try await APIClient.shared.getTrip(id: trip.id)
        } catch {
            errorMessage = error.localizedDescription
        }
        isLoading = false
    }

    @MainActor
    func cancel() async -> Bool {
        do {
            try await APIClient.shared.cancelTrip(id: trip.id)
            trip = Trip(
                id: trip.id, userId: trip.userId,
                originLat: trip.originLat, originLng: trip.originLng, originName: trip.originName,
                destinationLat: trip.destinationLat, destinationLng: trip.destinationLng, destinationName: trip.destinationName,
                desiredArrivalAt: trip.desiredArrivalAt, warningMinutes: trip.warningMinutes,
                status: .cancelled,
                latestEtaSeconds: trip.latestEtaSeconds,
                nextPollAt: trip.nextPollAt,
                notificationSentAt: trip.notificationSentAt,
                createdAt: trip.createdAt,
                updatedAt: Date()
            )
            return true
        } catch {
            errorMessage = error.localizedDescription
            return false
        }
    }

    @MainActor
    func activate() async {
        do {
            try await APIClient.shared.activateTrip(id: trip.id)
            await refresh()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    var etaText: String {
        guard trip.latestEtaSeconds > 0 else { return "Calculating…" }
        let mins = (trip.latestEtaSeconds + 59) / 60
        return "\(mins) min"
    }

    var leaveAtText: String {
        let f = DateFormatter()
        f.timeStyle = .short
        return f.string(from: trip.shouldLeaveAt)
    }

    var arrivalText: String {
        let f = DateFormatter()
        f.timeStyle = .short
        f.dateStyle = trip.desiredArrivalAt.isToday ? .none : .medium
        return f.string(from: trip.desiredArrivalAt)
    }
}

private extension Date {
    var isToday: Bool { Calendar.current.isDateInToday(self) }
}
