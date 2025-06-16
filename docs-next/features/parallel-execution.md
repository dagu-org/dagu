# Parallel Execution

Dagu supports parallel execution of workflow steps, allowing you to optimize performance and resource utilization.

## Basic Parallel Execution

Steps without dependencies run in parallel by default:

```yaml
steps:
  - name: task1
    command: process1.sh
    
  - name: task2
    command: process2.sh
    
  - name: task3
    command: process3.sh
    
  # All three tasks run simultaneously
```

## Controlling Parallelism

### Max Active Steps

Limit concurrent step execution:

```yaml
maxActiveSteps: 3  # Maximum 3 steps run in parallel

steps:
  - name: download1
    command: wget file1.zip
    
  - name: download2
    command: wget file2.zip
    
  - name: download3
    command: wget file3.zip
    
  - name: download4
    command: wget file4.zip
    
  # Only 3 downloads run at once
```

### Dependencies

Control execution order with dependencies:

```yaml
steps:
  - name: setup
    command: prepare.sh
    
  - name: process_a
    command: process_a.sh
    depends: setup
    
  - name: process_b
    command: process_b.sh
    depends: setup
    
  - name: combine
    command: combine.sh
    depends:
      - process_a
      - process_b
```

## Parallel Patterns

### Fan-Out Pattern

Process multiple items in parallel:

```yaml
steps:
  - name: get items
    command: ls data/*.csv
    output: FILES
    
  - name: process file1
    command: process.py data/file1.csv
    depends: get items
    
  - name: process file2
    command: process.py data/file2.csv
    depends: get items
    
  - name: process file3
    command: process.py data/file3.csv
    depends: get items
```

### Dynamic Fan-Out

Generate parallel steps dynamically:

```yaml
steps:
  - name: list files
    command: |
      for file in data/*.csv; do
        echo "- name: process_$(basename $file .csv)"
        echo "  command: process.py $file"
        echo "  depends: list files"
      done > /tmp/dynamic_steps.yaml
    
  - name: run parallel
    run: /tmp/dynamic_steps.yaml
    depends: list files
```

### Map-Reduce Pattern

```yaml
maxActiveSteps: 5

steps:
  # Map phase - parallel processing
  - name: map_chunk1
    command: map.py --input data1.txt --output /tmp/map1.out
    
  - name: map_chunk2
    command: map.py --input data2.txt --output /tmp/map2.out
    
  - name: map_chunk3
    command: map.py --input data3.txt --output /tmp/map3.out
    
  # Reduce phase - combine results
  - name: reduce
    command: reduce.py --inputs /tmp/map*.out --output result.txt
    depends:
      - map_chunk1
      - map_chunk2
      - map_chunk3
```

## Resource Management

### CPU-Bound Tasks

Limit parallelism for CPU-intensive tasks:

```yaml
maxActiveSteps: 2  # Match CPU cores

steps:
  - name: encode_video1
    command: ffmpeg -i input1.mp4 output1.mp4
    
  - name: encode_video2
    command: ffmpeg -i input2.mp4 output2.mp4
    
  - name: encode_video3
    command: ffmpeg -i input3.mp4 output3.mp4
```

### IO-Bound Tasks

Higher parallelism for IO operations:

```yaml
maxActiveSteps: 10  # More parallel IO

steps:
  - name: download1
    command: curl -O https://example.com/file1
    
  - name: download2
    command: curl -O https://example.com/file2
    
  # ... more downloads
```

### Mixed Workloads

```yaml
steps:
  # Parallel downloads (IO-bound)
  - name: download_data
    command: download_all.sh
    
  - name: download_models
    command: download_models.sh
    
  # Sequential processing (CPU-bound)
  - name: process_data
    command: heavy_processing.py
    depends: download_data
    
  - name: apply_models
    command: apply_models.py
    depends:
      - download_models
      - process_data
```

## Conditional Parallelism

### Environment-Based

```yaml
env:
  - MAX_PARALLEL: ${MAX_PARALLEL:-5}

maxActiveSteps: ${MAX_PARALLEL}

steps:
  - name: task1
    command: process.sh input1
  # ... more tasks
```

### Time-Based

```yaml
steps:
  - name: check time
    command: |
      HOUR=$(date +%H)
      if [ $HOUR -lt 8 ] || [ $HOUR -gt 18 ]; then
        echo "10"  # More parallel at night
      else
        echo "3"   # Less parallel during business hours
      fi
    output: MAX_STEPS
    
  - name: update config
    command: |
      sed -i "s/maxActiveSteps: .*/maxActiveSteps: ${MAX_STEPS}/" dag.yaml
    depends: check time
```

## Parallel DAG Execution

### Multiple DAG Runs

Control concurrent DAG instances:

```yaml
maxActiveRuns: 1  # Only one instance at a time
schedule: "*/5 * * * *"  # Every 5 minutes
skipIfSuccessful: true  # Skip if already running
```

### Parallel Sub-DAGs

```yaml
steps:
  - name: region_us
    run: process_region.yaml
    params: "REGION=us-east-1"
    
  - name: region_eu
    run: process_region.yaml
    params: "REGION=eu-west-1"
    
  - name: region_asia
    run: process_region.yaml
    params: "REGION=ap-southeast-1"
    
  # All regions process in parallel
```
