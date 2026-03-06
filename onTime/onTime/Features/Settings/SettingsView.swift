import SwiftUI

struct SettingsView: View {
    @Environment(AppCoordinator.self) private var coordinator
    @Environment(NotificationManager.self) private var notificationManager
    @State private var showSignOutConfirm = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Notifications") {
                    HStack {
                        Text("Device token")
                        Spacer()
                        Text(shortToken)
                            .font(.footnote.monospaced())
                            .foregroundStyle(.secondary)
                    }
                    Button("Re-register for notifications") {
                        Task { _ = await notificationManager.requestPermission() }
                    }
                }

                Section("Account") {
                    Button(role: .destructive) {
                        showSignOutConfirm = true
                    } label: {
                        Label("Sign Out", systemImage: "rectangle.portrait.and.arrow.right")
                    }
                }

                Section("About") {
                    LabeledContent("Version", value: appVersion)
                    LabeledContent("Build", value: buildNumber)
                }
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { coordinator.dismissSheet() }
                }
            }
            .confirmationDialog("Sign out of onTime?", isPresented: $showSignOutConfirm, titleVisibility: .visible) {
                Button("Sign Out", role: .destructive) {
                    Task { await coordinator.signOut() }
                }
                Button("Cancel", role: .cancel) {}
            }
        }
    }

    private var shortToken: String {
        guard let token = notificationManager.deviceToken else { return "Not registered" }
        return String(token.prefix(8)) + "…"
    }

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "–"
    }

    private var buildNumber: String {
        Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "–"
    }
}
