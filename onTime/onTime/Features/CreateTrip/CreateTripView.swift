import MapKit
import SwiftUI

struct CreateTripView: View {
    @Environment(AppCoordinator.self) private var coordinator
    @Environment(LocationManager.self) private var locationManager
    @State private var viewModel = CreateTripViewModel()
    @State private var showOriginSearch = false
    @State private var showDestinationSearch = false

    var body: some View {
        @Bindable var viewModel = viewModel
        NavigationStack {
            Form {
                // MARK: - Route
                Section("Route") {
                    locationRow(
                        icon: "circle.fill",
                        iconColor: .blue,
                        label: "From",
                        value: viewModel.originName.isEmpty ? "Current Location" : viewModel.originName,
                        placeholder: true
                    ) { showOriginSearch = true }

                    locationRow(
                        icon: "mappin.circle.fill",
                        iconColor: .red,
                        label: "To",
                        value: viewModel.destinationItem == nil ? "Search destination" : viewModel.destinationName,
                        placeholder: viewModel.destinationItem == nil
                    ) { showDestinationSearch = true }
                }

                // MARK: - Arrival time
                Section("Arrival") {
                    DatePicker(
                        "Arrive by",
                        selection: $viewModel.desiredArrivalAt,
                        in: Date()...,
                        displayedComponents: [.date, .hourAndMinute]
                    )

                    Stepper(
                        "Warning: \(viewModel.warningMinutes) min",
                        value: $viewModel.warningMinutes,
                        in: 0...60,
                        step: 5
                    )
                }

                if let error = viewModel.errorMessage {
                    Section {
                        Text(error)
                            .foregroundStyle(.red)
                            .font(.footnote)
                    }
                }
            }
            .navigationTitle("New Trip")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { coordinator.dismissSheet() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if viewModel.isLoading {
                        ProgressView()
                    } else {
                        Button("Create") { createTrip() }
                            .fontWeight(.semibold)
                            .disabled(!viewModel.canCreate)
                    }
                }
            }
            .sheet(isPresented: $showOriginSearch) {
                LocationSearchView(
                    title: "Origin",
                    placeholder: "Search origin…",
                    selectedItem: $viewModel.originItem
                )
            }
            .sheet(isPresented: $showDestinationSearch) {
                LocationSearchView(
                    title: "Destination",
                    placeholder: "Search destination…",
                    selectedItem: $viewModel.destinationItem
                )
            }
            .onAppear {
                viewModel.prefillOrigin(from: locationManager)
            }
        }
    }

    private func locationRow(
        icon: String,
        iconColor: Color,
        label: String,
        value: String,
        placeholder: Bool,
        action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            HStack(spacing: 12) {
                Image(systemName: icon)
                    .foregroundStyle(iconColor)
                    .frame(width: 20)
                VStack(alignment: .leading, spacing: 2) {
                    Text(label)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Text(value)
                        .foregroundStyle(placeholder ? .secondary : .primary)
                        .lineLimit(1)
                }
                Spacer()
                Image(systemName: "chevron.right")
                    .font(.caption)
                    .foregroundStyle(.tertiary)
            }
        }
        .buttonStyle(.plain)
    }

    private func createTrip() {
        Task {
            do {
                let trip = try await viewModel.create()
                coordinator.dismissSheet()
                coordinator.navigate(to: .tripDetail(trip))
            } catch {
                viewModel.errorMessage = error.localizedDescription
            }
        }
    }
}
