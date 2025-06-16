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

## Best Practices

### 1. Resource Awareness

Consider system resources:

```yaml
# For 4-core system
maxActiveSteps: 4

steps:
  - name: cpu_task1
    command: cpu_intensive.py
    env:
      - OMP_NUM_THREADS: 1  # Limit per-task threads
```

### 2. Error Handling

Handle failures in parallel tasks:

```yaml
steps:
  - name: parallel1
    command: risky_operation1.sh
    continueOn:
      failure: true
      
  - name: parallel2
    command: risky_operation2.sh
    continueOn:
      failure: true
      
  - name: check results
    command: |
      # Check if any parallel task failed
      if [ ! -f /tmp/result1 ] || [ ! -f /tmp/result2 ]; then
        echo "Some tasks failed"
        exit 1
      fi
    depends:
      - parallel1
      - parallel2
```

### 3. Progress Monitoring

Track parallel execution:

```yaml
steps:
  - name: monitor
    command: |
      while true; do
        COMPLETED=$(ls /tmp/done-* 2>/dev/null | wc -l)
        echo "Progress: $COMPLETED/10 tasks completed"
        sleep 5
      done &
      echo $! > /tmp/monitor.pid
    
  - name: task1
    command: |
      do_work.sh
      touch /tmp/done-task1
    depends: monitor
    
  # ... more tasks
  
  - name: stop monitor
    command: kill $(cat /tmp/monitor.pid)
    depends: [task1, task2, ...]
```

### 4. Load Balancing

Distribute work evenly:

```yaml
steps:
  - name: split data
    command: |
      # Split input into equal chunks
      split -n 4 input.txt chunk_
    
  - name: process1
    command: process.py chunk_aa
    depends: split data
    
  - name: process2
    command: process.py chunk_ab
    depends: split data
    
  - name: process3
    command: process.py chunk_ac
    depends: split data
    
  - name: process4
    command: process.py chunk_ad
    depends: split data
```

## Performance Optimization

### Minimize Dependencies

```yaml
# Less optimal - sequential
steps:
  - name: step1
    command: cmd1
  - name: step2
    command: cmd2
    depends: step1
  - name: step3
    command: cmd3
    depends: step2

# More optimal - parallel where possible
steps:
  - name: independent1
    command: cmd1
  - name: independent2
    command: cmd2
  - name: combine
    command: cmd3
    depends: [independent1, independent2]
```

### Batch Operations

```yaml
# Instead of many small parallel tasks
steps:
  - name: batch process
    command: |
      # Process multiple files in one step
      for file in *.csv; do
        process.py "$file" &
      done
      wait
```

### Resource Pools

```yaml
steps:
  - name: create pool
    command: |
      # Create connection pool
      create_pool.py --size=10 --output=/tmp/pool.sock
    
  - name: worker1
    command: worker.py --pool=/tmp/pool.sock --id=1
    depends: create pool
    
  - name: worker2
    command: worker.py --pool=/tmp/pool.sock --id=2
    depends: create pool
    
  # Workers share connection pool
```