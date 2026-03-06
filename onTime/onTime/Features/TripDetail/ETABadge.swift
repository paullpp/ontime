import SwiftUI

struct ETABadge: View {
    let etaSeconds: Int
    let shouldLeaveAt: Date

    private var isUrgent: Bool { shouldLeaveAt.timeIntervalSinceNow < 15 * 60 }
    private var isPast: Bool { shouldLeaveAt < Date() }

    private var etaText: String {
        guard etaSeconds > 0 else { return "–" }
        let mins = (etaSeconds + 59) / 60
        return "\(mins) min"
    }

    private var leaveText: String {
        if isPast { return "Leave now!" }
        let diff = shouldLeaveAt.timeIntervalSinceNow
        if diff < 3600 {
            return "Leave in \(Int(diff / 60)) min"
        }
        let f = DateFormatter(); f.timeStyle = .short
        return "Leave at \(f.string(from: shouldLeaveAt))"
    }

    var body: some View {
        VStack(spacing: 2) {
            HStack(spacing: 4) {
                Image(systemName: "car.fill")
                Text(etaText)
                    .fontWeight(.bold)
            }
            .font(.title2)
            .foregroundStyle(isPast ? .red : .primary)

            Text(leaveText)
                .font(.caption)
                .foregroundStyle(isUrgent ? .red : .secondary)
                .fontWeight(isUrgent ? .semibold : .regular)
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 12)
        .background(.regularMaterial)
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .shadow(color: .black.opacity(0.1), radius: 4, y: 2)
    }
}
