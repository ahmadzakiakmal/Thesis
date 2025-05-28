import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
from matplotlib.backends.backend_pdf import PdfPages

# Get user input for L2 nodes
l2_nodes = input("Enter L2 node number: ")

# Function to load and process a benchmark file
def load_benchmark(file_path):
    try:
        df = pd.read_csv(file_path)
        # Remove the "Complete Workflow" rows
        df = df[df['Step'] != 'Complete Workflow']
        return df
    except FileNotFoundError:
        print(f"Warning: File {file_path} not found!")
        return None

# Load benchmark files for BFT configurations (3f+1)
# This follows the pattern 4, 7, 10, 13, 16, etc.
bft_configurations = [4, 7, 10, 13, 16]  # L1 node configurations to compare
dataframes = {}

for l1 in bft_configurations:
    file_path = f'benchmark_n_100_l1_{l1}_l2_{l2_nodes}.csv'
    df = load_benchmark(file_path)
    if df is not None:
        dataframes[f'{l1}-{l2_nodes}'] = df

if not dataframes:
    print("No valid benchmark files found. Please run benchmarks first.")
    exit(1)

# Function to calculate average L1 and L2 latencies
def calculate_layer_latencies(df):
    # L1 is just the "Commit Session" endpoint
    l1_latency = df[df['Step'] == 'Commit Session']['Latency_ms'].mean()
    
    # L2 is the average of all other endpoints (excluding "Commit Session")
    l2_steps = df[df['Step'] != 'Commit Session']
    # Calculate average of all L2 endpoints
    l2_latency_total = 0
    l2_endpoint_count = 0
    
    for step in l2_steps['Step'].unique():
        l2_latency_total += df[df['Step'] == step]['Latency_ms'].mean()
        l2_endpoint_count += 1
    
    l2_latency = l2_latency_total / l2_endpoint_count if l2_endpoint_count > 0 else 0
    
    return {
        'L1 (Commit)': l1_latency,
        'L2 (Avg)': l2_latency
    }

# Calculate the latencies for each configuration
latencies = {config: calculate_layer_latencies(df) for config, df in dataframes.items()}

# Create a DataFrame for plotting
configs = list(dataframes.keys())
# Sort configurations by L1 node count
configs.sort(key=lambda x: int(x.split('-')[0]))

l1_values = [latencies[config]['L1 (Commit)'] for config in configs]
l2_values = [latencies[config]['L2 (Avg)'] for config in configs]

# Define x-positions for the bars and bar width
ind = np.arange(len(configs))
bar_width = 0.7

# Calculate fault tolerance for each configuration (f = (n-1)/3)
fault_tolerance = {}
for config in configs:
    l1_nodes = int(config.split('-')[0])
    fault_tolerance[config] = (l1_nodes - 1) // 3

# Create the figure and axis with IEEE-style dimensions
fig, ax = plt.subplots(figsize=(7.16, 5.5))  # IEEE single column width is ~3.5 inches, so 7.16 inches at 300 DPI

# IEEE style colors - black, red, blue, dark red (limited palette for formal papers)
ieee_colors = ['#000000BF', '#ff0000', '#0000dd', '#d62728']

# Plot L2 bars first (bottom of stack)
l2_bars = ax.bar(ind, l2_values, bar_width, label='L2 (Avg)', color='#c6dbef', edgecolor='black', linewidth=0.8)

# Plot L1 bars on top
l1_bars = ax.bar(ind, l1_values, bar_width, bottom=l2_values, label='L1 (Commit)', color='#fc9272', edgecolor='black', linewidth=0.8)

# Add trend lines for all components with IEEE style
ax.plot(ind, l2_values, marker='o', markersize=5, color=ieee_colors[2], linewidth=1.2, label='L2 Trend')
ax.plot(ind, l1_values, marker='s', markersize=5, color=ieee_colors[1], linewidth=1.2, label='L1 Trend')

# Calculate total values and add trend line
total_values = np.array(l1_values) + np.array(l2_values)
ax.plot(ind, total_values, marker='^', markersize=5, color=ieee_colors[0], linewidth=1.5, label='Total Trend')

# Add labels and values to bars - IEEE papers prefer cleaner figures, so use smaller font
for i in range(len(configs)):
    # L2 value (middle of the L2 bar)
    if l2_values[i] > max(l2_values) * 0.1:  # Only add text if bar is tall enough
        ax.text(i, l2_values[i]/2, f'{l2_values[i]:.1f}', ha='center', va='center', 
                color='black', fontweight='normal', fontsize=14)
    
    # L1 value (middle of the L1 bar)
    if l1_values[i] > max(l1_values) * 0.1:  # Only add text if bar is tall enough
        ax.text(i, l2_values[i] + l1_values[i]/2, f'{l1_values[i]:.1f}', ha='center', va='center', 
                color='black', fontweight='normal', fontsize=14)
    
    # Total value (above the stacked bar)
    total = l1_values[i] + l2_values[i]
    ax.text(i, total + max(total_values)*0.03, f'{total:.1f}', ha='center', va='bottom', 
            color="#000", fontweight='bold', fontsize=14)

# Customize the plot with IEEE styling
ax.set_ylabel('Latency (ms)', fontsize=15, labelpad=10)
ax.set_xlabel('Node Configuration (L1-L2)', fontsize=15, labelpad=12)
# ax.set_title(f'Latency Analysis of BFT Node Configurations', fontsize=11)
ax.set_xticks(ind)
ax.set_xticklabels(configs, rotation=0, ha='right')

# Position the legend according to IEEE style
ax.legend(loc='upper left', fontsize=12, frameon=True, framealpha=0.6, edgecolor='lightgray')

# Add BFT fault tolerance info as annotations
# for i, config in enumerate(configs):
#     ax.annotate(f'f={fault_tolerance[config]}', 
#                 xy=(i, -0.06), 
#                 xycoords=('data', 'axes fraction'),
#                 ha='center', 
#                 va='top',
#                 fontsize=8)

# Add a second line below the x-axis label for IEEE style
# ax.set_xlabel(f'Node Configuration (L1-L2)\nf = Byzantine Fault Tolerance', fontsize=14)

# Add grid for better readability - IEEE prefers subtle grid lines
ax.grid(axis='y', linestyle='dotted', alpha=0.5, color='gray')

# Ensure y-axis starts at 0 (common in IEEE)
ax.set_ylim(bottom=0)

# Calculate appropriate upper y-limit with padding - IEEE style typically has tight boundaries
max_total = max(total_values)
ax.set_ylim(top=1000)  # 12% padding

# Set tick parameters to match IEEE style - smaller, cleaner ticks
ax.tick_params(axis='both', which='major', labelsize=12)
ax.tick_params(axis='both', which='minor', labelsize=12)

# Make sure all spines are visible but subtle
for spine in ax.spines.values():
    spine.set_visible(True)
    spine.set_linewidth(0.5)
    spine.set_color('black')

# Adjust layout for IEEE paper format
plt.tight_layout()
plt.subplots_adjust(left=0.15, bottom=0.25, right=0.98, top=0.92)

# Define output filenames with IEEE in the name
output_base = f'bft_l1_l2_comparison_ieee_l2_{l2_nodes}'
output_png = f'{output_base}.png'
output_pdf = f'{output_base}.pdf'

# Save the chart as PNG with IEEE-compatible resolution
plt.savefig(output_png, dpi=600, bbox_inches='tight')
print(f"IEEE-style PNG chart saved as: {output_png}")

# Save as PDF using PdfPages for more reliable PDF creation
try:
    with PdfPages(output_pdf) as pdf:
        pdf.savefig(fig, bbox_inches='tight')
    print(f"IEEE-style PDF chart saved as: {output_pdf}")
except Exception as e:
    print(f"Error saving PDF: {e}")
    print("Try installing the required dependencies with: pip install matplotlib")

# Show the plot
plt.show()

# Generate a more comprehensive data table
print("\nBFT Latency Analysis Table:")
value_df = pd.DataFrame({
    'Configuration': configs,
    'L1 Nodes': [int(c.split('-')[0]) for c in configs],
    'L2 Nodes': [int(c.split('-')[1]) for c in configs],
    'Fault Tolerance (f)': [fault_tolerance[c] for c in configs],
    'L2 Latency (ms)': l2_values,
    'L1 Latency (ms)': l1_values,
    'Total Latency (ms)': total_values,
    'L1 Percentage (%)': [(l1 / total) * 100 for l1, total in zip(l1_values, total_values)]
})
print(value_df.round(2))

# Save the values to a CSV file
values_csv = f'bft_latency_analysis_l2_{l2_nodes}.csv'
value_df.to_csv(values_csv, index=False)
print(f"\nComplete analysis saved to CSV: {values_csv}")