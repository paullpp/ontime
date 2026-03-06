import SwiftUI

struct TripRowView: View {
    let trip: Trip

    private var etaText: String {
        guard trip.latestEtaSeconds > 0 else { return "Calculating…" }
        let mins = (trip.latestEtaSeconds + 59) / 60
        return "\(mins) min drive"
    }

    private var leaveText: String {
        let leave = trip.shouldLeaveAt
        if leave < Date() { return "Leave now!" }
        let diff = leave.timeIntervalSinceNow
        if diff < 3600 {
            let mins = Int(diff / 60)
            return "Leave in \(mins) min"
        }
        let f = DateFormatter()
        f.timeStyle = .short
        return "Leave by \(f.string(from: leave))"
    }

    private var arrivalText: String {
        let f = DateFormatter()
        f.timeStyle = .short
        f.dateStyle = trip.desiredArrivalAt.isToday ? .none : .short
        return "Arrive \(f.string(from: trip.desiredArrivalAt))"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(trip.destinationName)
                        .font(.headline)
                    Text(arrivalText)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                StatusBadge(status: trip.status)
            }

            Divider()

            HStack {
                Label(etaText, systemImage: "car.fill")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                Spacer()
                Text(leaveText)
                    .font(.footnote)
                    .fontWeight(.medium)
                    .foregroundStyle(trip.shouldLeaveAt < Date() ? .red : .primary)
            }
        }
        .padding(.vertical, 4)
    }
}

struct StatusBadge: View {
    let status: Trip.Status

    var body: some View {
        Text(status.rawValue.capitalized)
            .font(.caption2)
            .fontWeight(.semibold)
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.15))
            .foregroundStyle(color)
            .clipShape(Capsule())
    }

    private var color: Color {
        switch status {
        case .active:    return .blue
        case .notified:  return .green
        case .cancelled: return .secondary
        case .expired:   return .orange
        }
    }
}

private extension Date {
    var isToday: Bool { Calendar.current.isDateInToday(self) }
}
