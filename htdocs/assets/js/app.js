var satsumaApp = angular.module('satsuma', [ 'ngUpload' ]);


satsumaApp.directive('g+signin', function() {
	return {
		restrict: 'E',
		template: '<span></span>',
		replace: true,
		link: function(scope, element, attrs) {

			// Set class.
			attrs.$set('class', 'g-signin');

			attrs.$set('data-clientid', attrs.clientid + '.apps.googleusercontent.com');

			// Some default values, based on prior versions of this directive
			var defaults = {
				callback: 'signinCallback',
				cookiepolicy: 'single_host_origin',
				requestvisibleactions: 'http://schemas.google.com/AddActivity',
				scope: 'https://www.googleapis.com/auth/plus.login https://www.googleapis.com/auth/userinfo.email',
				width: 'wide'
			};

			// Provide default values if not explicitly set
			angular.forEach(Object.getOwnPropertyNames(defaults), function(propName) {
				if (!attrs.hasOwnProperty('data-' + propName)) {
					attrs.$set('data-' + propName, defaults[propName]);
				}
			});

			// Asynchronously load the G+ SDK.
			(function() {
				var po = document.createElement('script');
				po.type = 'text/javascript';
				po.async = true;
				po.src = 'https://apis.google.com/js/client:plusone.js';
				var s = document.getElementsByTagName('script')[0];
				s.parentNode.insertBefore(po, s);
			})();
		}
	};
});

satsumaApp.config(['$routeProvider', '$locationProvider', function($routeProvider, $locationProvider) {
		// TODO: configure routes
		$locationProvider.html5Mode(true);

		$routeProvider.when('/', { templateUrl: '/assets/partials/main.html', controller: 'MainCtrl' });
		$routeProvider.when('/v/:uploadid', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });
		$routeProvider.when('/s/:sessionid', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });

		$routeProvider.otherwise({ redirectTo: '/' });
	}
]);

satsumaApp.controller('PDFViewCtrl', [ '$scope', '$routeParams', '$http', function($scope, $routeParams, $http) {
	console.log('PDFViewCtrl: new instance. id = ' + $routeParams.id);
	if ($routeParams.uploadid) {
		$scope.type = "viewer";
		$scope.id = $routeParams.uploadid;
	} else if ($routeParams.sessionid) {
		$scope.sessionId = $routeParams.sessionid;
		$scope.type = "session";
	};

	$scope.pageNum = 1;
	PDFJS.disableWorker = true;
	$scope.loadProgress = 0;
	$scope.fullscreen = false;
	$scope.pdfDoc = null;
	$scope.origScale = 1.0;
	$scope.scale = $scope.origScale;

	$scope.loadPDF = function(path) {
		PDFJS.getDocument(path).then(function(_pdfDoc) {
			$scope.scale = $scope.origScale;
			$scope.pdfDoc = _pdfDoc;
			$scope.renderPage($scope.pageNum, function(success) {
				$scope.loadProgress = 0;
				$scope.$apply();
			});
		}, function(message, exception) {
			console.log("PDF load error: " + message);
		}, function(progressData) {
			$scope.loadProgress = (100 * progressData.loaded) / progressData.total;
			$scope.loadProgress = Math.round($scope.loadProgress*100)/100;
			console.log('loadProgress = ' + $scope.loadProgress);
			$scope.$apply();
		});
	};

	$scope.renderPage = function(num, callback) {
		$scope.pdfDoc.getPage(num).then(function(page) {
			var viewport = page.getViewport($scope.scale);
			if ($scope.fullscreen) {
				var new_scale = Math.min($scope.scale * (window.screen.height / viewport.height), $scope.scale * (window.screen.width / viewport.width));
				viewport = page.getViewport(new_scale);
			}

			var canvas = document.getElementById('slide_canvas');
			var ctx = canvas.getContext('2d');

			canvas.height = viewport.height;
			canvas.width = viewport.width;

			page.render({ canvasContext: ctx, viewport: viewport }).then(
				function() {
					if (callback)
						callback(true);
				},
				function() {
					if (callback)
						callback(false);
				}
			);
		});
	};

	$scope.zoomIn = function() {
		$scope.scale *= 1.2;
		$scope.renderPage($scope.pageNum, null);
	};

	$scope.zoomOut = function() {
		$scope.scale /= 1.2;
		$scope.renderPage($scope.pageNum, null);
	};

	$scope.gotoPrev = function() {
		if ($scope.pageNum > 1) {
			$scope.pageNum--;
			$scope.renderPage($scope.pageNum, null);
		}
	};

	$scope.gotoNext = function() {
		if ($scope.pageNum < $scope.pdfDoc.numPages) {
			$scope.pageNum++;
			$scope.renderPage($scope.pageNum, null);
		}
	};

	switch ($scope.type) {
	case "viewer":
		// TODO: fetch information.
		$scope.loadPDF("/userdata/" + $scope.id + ".pdf");
		break;
	case "session":
		$http.get('/api/sessioninfo/' + $scope.sessionId).
		success(function(data, status, header, config) {
			$scope.title = data.title;
			$scope.id = data.upload_id;
			$scope.owner = data.owner;
			/*
			if (data.page) {
				$scope.pageNum = data.page;
			}
			*/
			$scope.loadPDF("/userdata/" + $scope.id + ".pdf");
			if ($scope.owner) {
				console.log('TODO: connect WebSocket');
				// TODO: connect to WebSocket.
			}
		});
		break;
	}
}]);

satsumaApp.controller('LoginCtrl', [ '$scope', '$http', '$rootScope', function($scope, $http, $rootScope) {
	$rootScope.checkedLoggedIn = false;
	$rootScope.loggedIn = false;

	$http.get('/api/loggedin').
	success(function(data, status, headers, config) {
		$rootScope.checkedLoggedIn = true;
		$rootScope.loggedIn = data.logged_in;
	});
}]);

satsumaApp.controller('MainCtrl', ['$scope', '$http', '$rootScope', function($scope, $http, $rootScope) {
	console.log('MainCtrl: new instance');

	$scope.error = null;
	$scope.uploads = [ ];
	$scope.sessions = [ ];
	$scope.loading_uploads = false;
	$scope.loading_sessions = false;

	window.signinCallback = function(authData) {
		$scope.error = null;
		console.log(authData);
		if (authData['access_token']) {
			$http.post('/api/connect', authData['code']).
			success(function(data, status, headers, config) {
				$rootScope.loggedIn = true;
				$scope.$broadcast("loggedIn");
			}).
			error(function(data, status, headers, config) {
				$scope.error = "Signing in failed. Please try again later.";
			});
		} else if (authData['error']) {
			if (authData['error'] != "immediate_failed") {
				$scope.error = "Signing in failed (" + authData['error'] + ").";
			}
		}
		$scope.$apply();
	};

	$scope.$on("loggedIn", function() {
		$scope.getUploads();
		$scope.getSessions();
	});

	$scope.getUploads = function() {
		$scope.loading_uploads = true;
		$http.get('/api/getuploads').
		success(function(data, status, headers, config) {
			$scope.uploads = data;
			$scope.loading_uploads = false;
		}).
		error(function() {
			$scope.loading_uploads = false;
		});
	};

	$scope.deleteUpload = function(uploadID) {
		$http.post('/api/delupload', { 'upload_id': uploadID }).
		success(function(data, status, headers, config) {
			$scope.getUploads();
		}).
		error(function() {
			console.log('deleting upload failed');
		});
	};

	$scope.getSessions = function() {
		$scope.loading_sessions = true;
		$http.get('/api/getsessions').
		success(function(data, status, headers, config) {
			$scope.sessions = data;
			$scope.loading_sessions = false;
		}).
		error(function() {
			$scope.loading_sessions = false;
		});
	};

	$scope.startSession = function(uploadID) {
		$http.post('/api/startsession', { "upload_id": uploadID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			console.log('starting session failed');
		});
	};

	$scope.stopSession = function(sessionID) {
		$http.post('/api/stopsession', { "session_id": sessionID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			console.log('stopping session failed');
		});
	};

	$scope.deleteSession = function(sessionID) {
		$http.post('/api/delsession', { "session_id": sessionID }).
		success(function(data, status, headers, config) {
			$scope.getSessions();
		}).
		error(function() {
			console.log('stopping session failed');
		});
	};

	$scope.closeError = function() {
		$scope.error = null;
	};

	$scope.signOut = function() {
		$http.post('/api/disconnect').
		success(function(data, status, headers, config) {
			$rootScope.loggedIn = false;
		}).
		error(function(data, status, headers, config) {
			$rootScope.loggedIn = false;
			console.log('disconnect failed: ' + data);
		});
	};

	$scope.uploadComplete = function(content, completed) {
		if (completed) {
			$scope.hideUpload();
			$scope.getUploads();
		}
	};

	$scope.hideUpload = function() {
		$scope.showUpload = false;
		$('#upload_form').get(0).reset();
	};

	$scope.openUpload = function() {
		$scope.showUpload = true;
	};

	if ($rootScope.loggedIn) {
		$scope.getUploads();
		$scope.getSessions();
	}
}]);
