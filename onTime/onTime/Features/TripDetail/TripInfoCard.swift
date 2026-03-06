import SwiftUI

struct TripInfoCard: View {
    let trip: Trip

    var body: some View {
        VStack(spacing: 0) {
            infoRow(
                icon: "mappin.circle",
                iconColor: .blue,
                label: "From",
                value: trip.hasOrigin ? trip.originName : "Current location"
            )
            Divider().padding(.leading, 48)
            infoRow(
                icon: "mappin.circle.fill",
                iconColor: .red,
                label: "To",
                value: trip.destinationName
            )
            Divider().padding(.leading, 48)
            infoRow(
                icon: "clock",
                iconColor: .orange,
                label: "Arrive by",
                value: formatArrival(trip.desiredArrivalAt)
            )
            if trip.warningMinutes > 0 {
                Divider().padding(.leading, 48)
                infoRow(
                    icon: "bell",
                    iconColor: .purple,
                    label: "Warning",
                    value: "\(trip.warningMinutes) min early"
                )
            }
        }
        .background(.regularMaterial)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    private func infoRow(icon: String, iconColor: Color, label: String, value: String) -> some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .font(.title3)
                .foregroundStyle(iconColor)
                .frame(width: 28)
            VStack(alignment: .leading, spacing: 1) {
                Text(label)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text(value)
                    .font(.subheadline)
                    .fontWeight(.medium)
            }
            Spacer()
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
    }

    private func formatArrival(_ date: Date) -> String {
        let f = DateFormatter()
        f.timeStyle = .short
        f.dateStyle = Calendar.current.isDateInToday(date) ? .none : .medium
        return f.string(from: date)
    }
}
