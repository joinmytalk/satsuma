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
		$routeProvider.when('/v/:id', { templateUrl: '/assets/partials/pdfviewer.html', controller: 'PDFViewCtrl' });

		$routeProvider.otherwise({ redirectTo: '/' });
	}
]);

satsumaApp.controller('PDFViewCtrl', [ '$scope', '$routeParams', function($scope, $routeParams) {
	console.log('PDFViewCtrl: new instance. id = ' + $routeParams.id);

	// TODO: implement

}]);

satsumaApp.controller('LoginCtrl', [ '$scope', '$http', '$rootScope', function($scope, $http, $rootScope) {
	$rootScope.checkedLoggedIn = false;
	$rootScope.loggedIn = false;

	$http.get('/api/logged_in').
	success(function(data, status, headers, config) {
		$rootScope.checkedLoggedIn = true;
		$rootScope.loggedIn = data.logged_in;
	});
}]);

satsumaApp.controller('MainCtrl', ['$scope', '$http', '$rootScope', function($scope, $http, $rootScope) {
	console.log('MainCtrl: new instance');

	$scope.error = null;
	$scope.uploads = [ ];
	$scope.loading = false;

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
	});

	$scope.getUploads = function() {
		$scope.loading = true;
		$http.get('/api/getuploads').
		success(function(data, status, headers, config) {
			$scope.uploads = data;
			$scope.loading = false;
		}).
		error(function() {
			$scope.loading = false;
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
	}
}]);
