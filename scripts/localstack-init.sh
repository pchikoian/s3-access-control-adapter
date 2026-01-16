#!/bin/bash

# Create test buckets for different tenants
awslocal s3 mb s3://tenant-001-data
awslocal s3 mb s3://tenant-001-uploads
awslocal s3 mb s3://tenant-002-data
awslocal s3 mb s3://shared-bucket

# Add some test objects
echo "Hello from tenant-001" | awslocal s3 cp - s3://tenant-001-data/test.txt
echo "Hello from tenant-002" | awslocal s3 cp - s3://tenant-002-data/test.txt

echo "LocalStack S3 initialized with test buckets"
