import MapKit
import SwiftUI

struct LocationSearchView: View {
    let title: String
    let placeholder: String
    @Binding var selectedItem: MKMapItem?
    @Environment(\.dismiss) private var dismiss

    @State private var query = ""
    @State private var results: [MKMapItem] = []
    @State private var isSearching = false

    var body: some View {
        NavigationStack {
            List {
                if results.isEmpty && !query.isEmpty && !isSearching {
                    Text("No results for "\(query)"")
                        .foregroundStyle(.secondary)
                } else {
                    ForEach(results, id: \.self) { item in
                        Button {
                            selectedItem = item
                            dismiss()
                        } label: {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(item.name ?? "Unknown")
                                    .fontWeight(.medium)
                                    .foregroundStyle(.primary)
                                if let subtitle = item.placemark.title, subtitle != item.name {
                                    Text(subtitle)
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                        .lineLimit(1)
                                }
                            }
                            .padding(.vertical, 2)
                        }
                    }
                }
            }
            .listStyle(.plain)
            .searchable(text: $query, placement: .navigationBarDrawer(displayMode: .always), prompt: placeholder)
            .onChange(of: query) { _, new in
                Task { await search(new) }
            }
            .navigationTitle(title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
            .overlay {
                if isSearching {
                    ProgressView()
                }
            }
        }
    }

    private func search(_ text: String) async {
        guard text.count >= 2 else { results = []; return }
        isSearching = true
        let req = MKLocalSearch.Request()
        req.naturalLanguageQuery = text
        req.resultTypes = [.pointOfInterest, .address]
        let response = try? await MKLocalSearch(request: req).start()
        results = response?.mapItems ?? []
        isSearching = false
    }
}
