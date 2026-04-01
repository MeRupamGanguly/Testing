
curl -X POST http://localhost:8080/api/v1/storage/upload \
  -F "bucket=mytestbucket" \
  -F "file=@testfile.txt"


  curl "http://localhost:8080/api/v1/storage/list?bucket=mytestbucket"

  curl -X PUT http://localhost:8080/api/v1/storage/modify \
  -F "bucket=mytestbucket" \
  -F "file=@testfile.txt"


  curl "http://localhost:8080/api/v1/storage/download?bucket=mytestbucket&filepath=testfile.txt" --output downloaded.txt


  curl -X POST http://localhost:8080/api/v1/storage/signed-url \
  -H "Content-Type: application/json" \
  -d '{"bucket":"mytestbucket", "filepath":"testfile.txt", "duration_minutes":10}'



  curl -X POST http://localhost:8080/api/v1/storage/delete \
  -H "Content-Type: application/json" \
  -d '{"bucket":"mytestbucket", "filepath":"testfile.txt"}'
