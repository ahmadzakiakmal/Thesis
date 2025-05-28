import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import os
from matplotlib.backends.backend_pdf import PdfPages

# Get L2 nodes from user input
l2_nodes = input("Enter number of L2 nodes: ")

# Function to load and process a benchmark file
def load_benchmark(file_path):
    if not os.path.exists(file_path):
        print(f"Warning: File {file_path} not found")
        return None
    
    df = pd.read_csv(file_path)
    # Filter out the "Complete Workflow" rows
    df = df[df['Step'] != 'Complete Workflow']
    return df

# Load the three benchmark files
l1_configurations = [4, 7, 10, 13, 16]  # Different L1 node configurations to compare
dataframes = {}

for l1 in l1_configurations:
    file_path = f'benchmark_n_100_l1_{l1}_l2_{l2_nodes}.csv'
    df = load_benchmark(file_path)
    if df is not None:
        dataframes[f'{l1}-{l2_nodes}'] = df

if not dataframes:
    print("No valid benchmark files found. Please run benchmarks first.")
    exit(1)

# Function to calculate average latencies by endpoint
def get_avg_latencies(df):
    return df.groupby('Step')['Latency_ms'].mean().to_dict()

# Get average latencies for each configuration
avg_latencies = {config: get_avg_latencies(df) for config, df in dataframes.items()}

# Define the order of steps in the workflow
step_order = [
    'Create Package',
    'Start Session',
    'Scan Package',
    'Validate Package',
    'Quality Check',
    'Label Package',
    'Commit Session'  # This will be the only red one (L1)
]

# Prepare data for plotting
configs = list(dataframes.keys())
# Custom sort to ensure L1 configurations are in ascending order
configs.sort(key=lambda x: int(x.split('-')[0]))  # Sort by L1 node count

# Create a DataFrame with the right structure for stacked bar chart
plot_data = pd.DataFrame({
    config: [avg_latencies[config].get(step, 0) for step in step_order]
    for config in configs
}, index=step_order)

# Calculate fault tolerance for each configuration (f = (n-1)/3)
fault_tolerance = {}
for config in configs:
    l1_nodes = int(config.split('-')[0])
    fault_tolerance[config] = (l1_nodes - 1) // 3

# Create a blue color palette for L2 operations and red for L1
# More distinct blue shades
blue_shades = [
    '#08519c',  # Very dark blue
    '#2171b5',  # Dark blue
    '#4292c6',  # Medium-dark blue
    '#6baed6',  # Medium blue
    '#9ecae1',  # Medium-light blue
    '#c6dbef',  # Light blue (base)
]

# Map steps to colors
step_colors = {}
for i, step in enumerate(step_order[:-1]):  # All except the last
    step_colors[step] = blue_shades[i % len(blue_shades)]

# Set the L1 step (Commit Session) to red
step_colors['Commit Session'] = '#fc9272'  # Red/salmon color for L1

# Create the figure with IEEE dimensions
fig, ax = plt.subplots(figsize=(7.16, 5))  # Using double-column width

# Plot stacked bars
bottom = np.zeros(len(configs))
total_heights = np.zeros(len(configs))

for step in step_order:
    values = [plot_data.loc[step, config] for config in configs]
    bars = ax.bar(configs, values, bottom=bottom, label=step, 
           color=step_colors[step], 
           edgecolor='black', linewidth=0.5,
           width=0.6)
    bottom += values
    total_heights += values

# Add a trend line for the total latency - IEEE style with clear markers
ax.plot(configs, total_heights, 'ko-', linewidth=1.5, markersize=6, label='Total Latency')

# IEEE-style font sizes
SMALL_SIZE = 8    # Minor elements
MEDIUM_SIZE = 11   # Most elements (as requested for legend)
LARGE_SIZE = 12   # Axis labels

# Customize the plot with IEEE styling
ax.set_ylabel('Latency (ms)', fontsize=15, labelpad=8)
ax.set_xlabel(f'Node Configuration (L1-L2)', fontsize=15, labelpad=8)
# ax.set_title(f'Latency Analysis of BFT Node Configurations (L2={l2_nodes})', fontsize=LARGE_SIZE)

# Set tick parameters with IEEE sizing
ax.tick_params(axis='both', which='major', labelsize=12)
ax.tick_params(axis='both', which='minor', labelsize=12)

# Place the legend in the top left corner
legend = ax.legend(fontsize=10, 
                  frameon=True, framealpha=0.6, edgecolor='lightgray',
                  ncol=1, loc='upper left')
legend.get_title().set_fontsize(10)  # Set legend title size

# Add fault tolerance annotations below the x-axis
# for i, config in enumerate(configs):
#     ax.annotate(f'f={fault_tolerance[config]}', 
#                 xy=(i, -0.05), xycoords=('data', 'axes fraction'),
#                 ha='center', va='top', fontsize=8)

# Add total latency values above each bar - smaller IEEE-style text
for i, total in enumerate(total_heights):
    ax.text(i, total + max(total_heights)*0.02, f'{total:.1f}', 
            ha='center', fontsize=14, fontweight='bold')

# Add grid lines (subtle, IEEE style)
ax.grid(axis='y', linestyle='dotted', alpha=0.5, color='gray')

# Ensure y-axis starts at 0 (standard in IEEE)
ax.set_ylim(bottom=0)

# Calculate appropriate upper y-limit with minimal padding (IEEE style)
ax.set_ylim(top=3100)  # 10% padding

# Make sure all spines are visible but subtle (IEEE style)
for spine in ax.spines.values():
    spine.set_visible(True)
    spine.set_linewidth(0.5)
    spine.set_color('black')

# With the legend inside, we can use standard tight layout
plt.tight_layout()

# Define output filenames with IEEE designation
output_base = f'ieee_stacked_latency_l1_l2_{l2_nodes}'
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

# Create a detailed data table for the paper
data_table = pd.DataFrame()

# Add configuration details
data_table['Configuration'] = configs
data_table['L1 Nodes'] = [int(c.split('-')[0]) for c in configs]
data_table['L2 Nodes'] = [int(c.split('-')[1]) for c in configs]
data_table['Fault Tolerance (f)'] = [fault_tolerance[c] for c in configs]

# Add latency for each step
for step in step_order:
    data_table[f'{step} (ms)'] = [plot_data.loc[step, config] for config in configs]

# Add total latency
data_table['Total Latency (ms)'] = total_heights

# Add percentage contribution of L1 (commit) to the total
commit_latencies = [plot_data.loc['Commit Session', config] for config in configs]
data_table['L1 Commit (%)'] = [(commit/total)*100 for commit, total in zip(commit_latencies, total_heights)]

print("\nDetailed Analysis Table for IEEE Paper:")
print(data_table.round(2))

# Save the values to a CSV file
table_csv = f'ieee_latency_analysis_l2_{l2_nodes}.csv'
data_table.to_csv(table_csv, index=False)
print(f"\nComplete analysis saved to CSV: {table_csv}")

# Also generate LaTeX code for the table (useful for IEEE papers)
try:
    latex_table = data_table.round(1).to_latex(index=False)
    with open(f'ieee_latency_table_l2_{l2_nodes}.tex', 'w') as f:
        f.write(latex_table)
    print(f"LaTeX table saved to: ieee_latency_table_l2_{l2_nodes}.tex")
except Exception as e:
    print(f"Could not generate LaTeX table: {e}")