pipeline {
  agent any

  options {
    timestamps()
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
      }
    }

    stage('Build') {
      steps {
        sh 'CGO_ENABLED=0 go build -o bin/server ./cmd/server'
      }
    }
  }

  post {
    always {
      archiveArtifacts artifacts: 'coverage.out', allowEmptyArchive: true
    }
  }
}
