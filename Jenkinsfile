pipeline {
  agent {
    kubernetes {
      defaultContainer 'golang'
      yaml '''
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: golang
      image: golang:1.24-bookworm
      command:
        - sleep
      args:
        - 99d
      tty: true
      resources:
        requests:
          cpu: 200m
          memory: 256Mi
        limits:
          cpu: "1"
          memory: 1Gi
'''
    }
  }

  options {
    timeout(time: 10, unit: 'MINUTES')
    disableConcurrentBuilds()
    buildDiscarder(logRotator(numToKeepStr: '20'))
  }

  stages {
    stage('Verify formatting') {
      steps {
        sh 'test -z "$(gofmt -l .)"'
      }
    }

    stage('Vet') {
      steps {
        sh 'go vet ./...'
      }
    }

    stage('Test') {
      steps {
        sh 'go test -race -coverprofile=coverage.out ./...'
        archiveArtifacts artifacts: 'coverage.out'
      }
    }

    stage('Build') {
      steps {
        sh 'CGO_ENABLED=0 go build -buildvcs=false -o bin/server ./cmd/server'
      }
    }
  }

}
