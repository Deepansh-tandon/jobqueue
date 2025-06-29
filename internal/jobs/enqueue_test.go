package jobs

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/go-redis/redis/v8"
    "github.com/stretchr/testify/assert"
  
    "testing"
    "time"
)

func TestEnqueueJob(t *testing.T) {
    fmt.Println("=== Starting EnqueueJob Test ===")
    
    // Get Redis URL from environment variable
    redisURL := "redis://:0A14chHYQTL7V9wSqJMUx1cWat505Qx0@redis-19161.c266.us-east-1-3.ec2.redns.redis-cloud.com:19161"
    if redisURL == "" {
        fmt.Println("❌ REDIS_URL environment variable not set, skipping test")
        t.Skip("REDIS_URL environment variable not set, skipping test")
    }
    fmt.Printf("✅ Using Redis URL: %s\n", redisURL)

    // Parse Redis URL to create client
    fmt.Println("🔧 Parsing Redis URL...")
    opts, err := redis.ParseURL(redisURL)
    if err != nil {
        fmt.Printf("❌ Failed to parse Redis URL: %v\n", err)
        t.Fatalf("Failed to parse Redis URL: %v", err)
    }
    fmt.Println("✅ Redis URL parsed successfully")

    fmt.Println("🔧 Creating Redis client...")
    rdb := redis.NewClient(opts)
    defer rdb.Close()
    fmt.Println("✅ Redis client created")

    // Test queue name
    queueName := "queue:priority:1"
    fmt.Printf("📋 Using queue name: %s\n", queueName)
    
    // Clean up any existing data in the test queue
    fmt.Printf("🧹 Cleaning up existing data in queue: %s\n", queueName)
    deletedCount, err := rdb.Del(context.Background(), queueName).Result()
    if err != nil {
        fmt.Printf("⚠️  Warning: Could not clean up queue: %v\n", err)
    } else {
        fmt.Printf("✅ Cleaned up %d existing items from queue\n", deletedCount)
    }

    // Create test job
    fmt.Println("📝 Creating test job...")
    job := Job{
        Type:    "email",
        Payload: map[string]interface{}{"to": "a@b.com"},
    }
    fmt.Printf("✅ Job created - Type: %s, Payload: %+v\n", job.Type, job.Payload)

    // Enqueue the job
    fmt.Println("📤 Enqueueing job...")
    err = EnqueueJob(rdb, job)
    if err != nil {
        fmt.Printf("❌ Failed to enqueue job: %v\n", err)
        assert.NoError(t, err)
        return
    }
    fmt.Println("✅ Job enqueued successfully")

    // Check queue length
    fmt.Println("📊 Checking queue length...")
    queueLength, err := rdb.LLen(context.Background(), queueName).Result()
    if err != nil {
        fmt.Printf("⚠️  Warning: Could not get queue length: %v\n", err)
    } else {
        fmt.Printf("✅ Queue length: %d\n", queueLength)
    }

    // Pop the job back
    fmt.Println("📥 Popping job from queue...")
    res, err := rdb.RPop(context.Background(), queueName).Result()
    if err != nil {
        fmt.Printf("❌ Failed to pop job from queue: %v\n", err)
        assert.NoError(t, err)
        return
    }
    fmt.Printf("✅ Job popped from queue. Raw data length: %d bytes\n", len(res))

    // Unmarshal the job
    fmt.Println("🔍 Unmarshaling job data...")
    var popped Job
    err = json.Unmarshal([]byte(res), &popped)
    if err != nil {
        fmt.Printf("❌ Failed to unmarshal job: %v\n", err)
        assert.NoError(t, err)
        return
    }
    fmt.Printf("✅ Job unmarshaled successfully\n")

    // Verify job details
    fmt.Println("🔍 Verifying job details...")
    fmt.Printf("   - Job Type: %s (expected: email)\n", popped.Type)
    fmt.Printf("   - Job ID: %s\n", popped.ID)
    fmt.Printf("   - Job Payload: %+v\n", popped.Payload)
    fmt.Printf("   - Job Created At: %s\n", popped.CreatedAt.Format(time.RFC3339))
    fmt.Printf("   - Job Max Retries: %d\n", popped.MaxRetries)
    fmt.Printf("   - Job Retry Count: %d\n", popped.RetryCount)

    // Assertions
    fmt.Println("✅ Running assertions...")
    assert.Equal(t, "email", popped.Type, "Job type should match")
    assert.WithinDuration(t, time.Now(), popped.CreatedAt, 5*time.Second, "Job creation time should be recent")
    assert.NotEmpty(t, popped.ID, "Job ID should not be empty")
    assert.Equal(t, 3, popped.MaxRetries, "Default max retries should be 3")
    assert.Equal(t, 0, popped.RetryCount, "Initial retry count should be 0")
    
    fmt.Println("=== EnqueueJob Test Completed Successfully ===")
}
