import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
from matplotlib.backends.backend_pdf import PdfPages

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
l2_nodes = input("Enter L2 node number (e.g., 1): ")
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
    l2_latency = l2_steps['Latency_ms'].mean()
    
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

# Create the figure and axis with IEEE-style dimensions
fig, ax = plt.subplots(figsize=(7, 5))

# Set up bar properties
bar_width = 0.425
index = np.arange(len(configs))

# Plot the bars
l2_bars = ax.bar(index - bar_width/2, l2_values, bar_width, 
                 label='Layer 2 (Avg)', color='#c6dbef', edgecolor='black', linewidth=0.8)
l1_bars = ax.bar(index + bar_width/2, l1_values, bar_width, 
                 label='Layer 1 (Commit)', color='#fc9272', edgecolor='black', linewidth=0.8)

# Add labels and values to bars
for i, v in enumerate(l2_values):
    ax.text(i - bar_width/2, v + max(l2_values)*0.01, f'{v:.1f}', 
            ha='center', va='bottom', fontsize=10, fontweight='bold')

for i, v in enumerate(l1_values):
    ax.text(i + bar_width/2, v + max(l1_values)*0.01, f'{v:.1f}', 
            ha='center', va='bottom', fontsize=10, fontweight='bold')

# Customize plot appearance
ax.set_xlabel('Node Configuration (L1-L2)', fontsize=14, labelpad=10)
ax.set_ylabel('Latency (ms)', fontsize=14, labelpad=10)
ax.set_xticks(index)
ax.set_yticklabels(configs, fontsize=12)
ax.set_xticklabels(configs, fontsize=12)

# Add grid for better readability
ax.yaxis.grid(True, linestyle='--', alpha=0.7, zorder=0)

# Add fault tolerance annotations
# for i, config in enumerate(configs):
#     l1_nodes = int(config.split('-')[0])
#     f_value = (l1_nodes - 1) // 3
    # ax.annotate(f'f={f_value}', xy=(i, -0.05), xycoords=('data', 'axes fraction'),
    #             ha='center', va='top', fontsize=9)

# Add legend
ax.legend(fontsize=12, loc='upper left')

# Ensure y-axis starts at 0
ax.set_ylim(bottom=0, top=500)

# Adjust layout
plt.tight_layout()
plt.subplots_adjust(bottom=0.15)

# Save the figure as PDF
output_file = f'l1_vs_l2_comparison_l2_{l2_nodes}.pdf'
with PdfPages(output_file) as pdf:
    pdf.savefig(fig, bbox_inches='tight')
print(f"PDF chart saved as: {output_file}")

# Also save as PNG for quick viewing
plt.savefig(f'l1_vs_l2_comparison_l2_{l2_nodes}.png', dpi=300, bbox_inches='tight')

# Show the plot
# plt.show()