import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

# Load the data
n = input("amount of requests: ")
l1_nodes = input("l1 nodes: ")
l2_nodes = input("l2 nodes: ")
file_name = f"benchmark_n_{n}_l1_{l1_nodes}_l2_{l2_nodes}.csv"
df = pd.read_csv('./' + file_name)

df = df.iloc[:]

# Convert latency to numeric
df['Latency_ms'] = pd.to_numeric(df['Latency_ms'])

# Create a mapping for cleaner labels
endpoint_mapping = {
    '/session/test-package': 'Create Package',
    '/session/start': 'Start Session',
    'session/:id/scan/:packageId': 'Scan',
    'session/:id/validate': 'Validate',
    'session/:id/qc': 'QC',
    'session/:id/label': 'Label',
    'commit/:id': "Commit L1",
    'complete-workflow': 'Whole Workflow'
}

# Apply the mapping to create a new column
df['Clean_Endpoint'] = df['Endpoint'].map(endpoint_mapping)

# Define the order based on workflow steps
workflow_order = [
    'Create Package',
    'Start Session',
    'Scan',
    'Validate',
    'QC',
    'Label',
    'Commit L1',
    # 'Whole Workflow',
]

# Create a categorical type with our custom order
df['Clean_Endpoint'] = pd.Categorical(df['Clean_Endpoint'], categories=workflow_order, ordered=True)

# Sort the dataframe based on our ordered category
df = df.sort_values('Clean_Endpoint')

# Create a figure and axis with IEEE-style dimensions
fig = plt.figure(figsize=(6, 4))
ax = fig.add_subplot(111)

# IEEE style colors
colors = ['#000', '#ff0000', 'tab:blue', '#d62728']

# Create boxplot with custom style
boxprops = dict(linestyle='-', linewidth=.8, color=colors[0])
whiskerprops = dict(linestyle='-', linewidth=.8, color=colors[0])
medianprops = dict(linestyle='-', linewidth=1, color=colors[1])
capprops = dict(linestyle='-', linewidth=.8, color=colors[0])
flierprops = dict(marker='o', markerfacecolor='none', markersize=5, 
                 markeredgecolor=colors[0], alpha=0.7)

boxplot = df.boxplot(column='Latency_ms', by='Clean_Endpoint', ax=ax,
                    boxprops=boxprops, whiskerprops=whiskerprops,
                    medianprops=medianprops, capprops=capprops,
                    flierprops=flierprops,
                    grid=True, showmeans=True, meanprops={'marker':'', 'markerfacecolor':'black', 'markeredgecolor':'black'})

# Customize grid
ax.grid(True, linestyle='dashed', alpha=0.7)

# Set tight margins like the IEEE example
plt.xlabel('Endpoint')
plt.ylabel('Latency (ms)', rotation=90, labelpad=10)

# Set appropriate y-axis ticks
max_latency = df['Latency_ms'].max()
y_max = int(500)  # Round up to nearest 50
step_size = 100  # Divide range into 5 steps (adjust as needed)

plt.ylim(0, y_max)
plt.yticks(np.arange(0, y_max + step_size, step_size))
# set y axis fontsize
plt.tick_params(axis='y', labelsize=10)
plt.xticks(fontsize=10)

# Remove automatic titles
plt.title(f"", fontsize=10)
plt.suptitle('')  # Remove the automatic title

# Adjust layout to match IEEE style
plt.subplots_adjust(left=0.155, bottom=0.25, right=0.98, top=0.92)

# Rotate x labels for better readability
plt.xticks(rotation=30, ha='right')

# Save figure with IEEE-compatible format
plt.savefig(f"chart_n_{n}_l1_{l1_nodes}_l2_{l2_nodes}.pdf", format='pdf', dpi=1000)
plt.savefig(f"chart_n_{n}_l1_{l1_nodes}_l2_{l2_nodes}.png", format='png', dpi=300)

print("Generated plots at:")
# print(f"chart_n_{n}_l1_{l1_nodes}_l2_{l2_nodes}.pdf")
print(f"chart_n_{n}_l1_{l1_nodes}_l2_{l2_nodes}.png")