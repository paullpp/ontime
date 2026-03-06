import SwiftUI

struct TripDetailView: View {
    @State private var viewModel: TripDetailViewModel
    @State private var showCancelConfirm = false
    @Environment(AppCoordinator.self) private var coordinator

    init(trip: Trip) {
        _viewModel = State(initialValue: TripDetailViewModel(trip: trip))
    }

    var body: some View {
        ZStack(alignment: .bottom) {
            MapContainerView(trip: viewModel.trip)
                .ignoresSafeArea(edges: .top)

            VStack(spacing: 12) {
                // ETA badge centered over the map
                ETABadge(
                    etaSeconds: viewModel.trip.latestEtaSeconds,
                    shouldLeaveAt: viewModel.trip.shouldLeaveAt
                )

                TripInfoCard(trip: viewModel.trip)
                    .padding(.horizontal, 16)

                // Action buttons
                HStack(spacing: 12) {
                    if viewModel.trip.status == .active {
                        Button(role: .destructive) {
                            showCancelConfirm = true
                        } label: {
                            Label("Cancel Trip", systemImage: "xmark.circle")
                                .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(.bordered)
                    } else if viewModel.trip.status != .active {
                        Button {
                            Task { await viewModel.activate() }
                        } label: {
                            Label("Re-activate", systemImage: "arrow.clockwise")
                                .frame(maxWidth: .infinity)
                        }
                        .buttonStyle(.borderedProminent)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.bottom, 8)
            }
            .padding(.top, 12)
            .background(
                RoundedRectangle(cornerRadius: 24)
                    .fill(.background)
                    .ignoresSafeArea(edges: .bottom)
            )
        }
        .navigationTitle(viewModel.trip.destinationName)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                if viewModel.isLoading {
                    ProgressView()
                } else {
                    Button { Task { await viewModel.refresh() } } label: {
                        Image(systemName: "arrow.clockwise")
                    }
                }
            }
        }
        .alert("Error", isPresented: .constant(viewModel.errorMessage != nil)) {
            Button("OK") { viewModel.errorMessage = nil }
        } message: {
            Text(viewModel.errorMessage ?? "")
        }
        .confirmationDialog("Cancel this trip?", isPresented: $showCancelConfirm, titleVisibility: .visible) {
            Button("Cancel Trip", role: .destructive) {
                Task {
                    let ok = await viewModel.cancel()
                    if ok { coordinator.pop() }
                }
            }
            Button("Keep Trip", role: .cancel) {}
        }
        // Poll for ETA updates while the view is visible (every 30s as a UI refresh).
        .task {
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(30))
                if viewModel.trip.isActive {
                    await viewModel.refresh()
                }
            }
        }
    }
}
